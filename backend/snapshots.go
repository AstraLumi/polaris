package main

// Daily copies of each story's database, made by the server on its own, for
// the day something goes wrong that nobody exported before: a bad edit, a
// mass delete, an upgrade regretted.
//
//	<DATA_DIR>/stories/<id>/backups/auto/snap-20261110-031500-s46.db
//
//   - one a day at most, and only when the story changed since the last;
//   - the newest 7 are kept, then the newest of each earlier week, 4 weeks
//     back; older ones are removed;
//   - the database only, not the pictures: those are never rewritten, and a
//     copy of them a day would fill the disk. A picture deleted since is
//     missing from a copy that is brought back;
//   - on the same disk as the story, so they are no defence against losing
//     the disk. Export is for that.
//
// A copy is brought back as a new story beside the present one ("Earlier
// copies" in the story picker), so nothing is ever overwritten.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

const (
	snapshotEvery   = 20 * time.Hour // "a day", with room for the hourly look
	snapshotDaily   = 7
	snapshotWeekly  = 4
	snapshotStamp   = "20060102-150405"
	snapshotLookGap = time.Hour
)

// A variable so the tests can move the clock.
var snapshotNow = time.Now

var snapshotName = regexp.MustCompile(`^snap-(\d{8}-\d{6})-s(\d+)\.db$`)

type storySnapshot struct {
	Name    string `json:"name"`
	TakenAt string `json:"taken_at"` // UTC, SQLite format, like every other stamp
	Size    int64  `json:"size"`
	Schema  string `json:"schema_version"`
	at      time.Time
}

func (m *storyManager) snapshotsDir(id string) string {
	return filepath.Join(m.backupsDir(id), "auto")
}

// snapshots lists a story's copies, newest first.
func (m *storyManager) snapshots(id string) []storySnapshot {
	entries, _ := os.ReadDir(m.snapshotsDir(id))
	out := []storySnapshot{}
	for _, e := range entries {
		match := snapshotName.FindStringSubmatch(e.Name())
		if match == nil || e.IsDir() {
			continue
		}
		at, err := time.ParseInLocation(snapshotStamp, match[1], time.UTC)
		if err != nil {
			continue
		}
		s := storySnapshot{Name: e.Name(), TakenAt: at.Format("2006-01-02 15:04:05"), Schema: match[2], at: at}
		if info, err := e.Info(); err == nil {
			s.Size = info.Size()
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].at.After(out[j].at) })
	return out
}

// snapshotStory makes today's copy when one is due. made is false when the
// last is recent enough or the story hasn't changed since.
func (m *storyManager) snapshotStory(id string) (made bool, err error) {
	if !storyIDPattern.MatchString(id) {
		return false, errNoStory
	}
	dbPath := m.dbPathFor(id)
	info, err := os.Stat(dbPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil // never opened: nothing to copy
		}
		return false, err
	}
	now := snapshotNow().UTC()
	if have := m.snapshots(id); len(have) > 0 {
		last := have[0].at
		// A commit touches the file, so one untouched since the last copy
		// holds nothing new.
		if now.Sub(last) < snapshotEvery || !info.ModTime().UTC().After(last) {
			return false, nil
		}
	}

	// The story's own connections when it is open, one of its own otherwise
	// (which upgrades nothing: the copy is of the story as it is).
	app, done := m.peek(id)
	defer done()
	var db *sql.DB
	if app != nil {
		db = app.db
	} else {
		if db, err = sql.Open("sqlite", sqliteDSN(dbPath)); err != nil {
			return false, err
		}
		defer db.Close()
	}
	schema := storedSchema(db)
	if schema == "" {
		return false, nil // not a story yet
	}
	dir := m.snapshotsDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	final := filepath.Join(dir, fmt.Sprintf("snap-%s-s%s.db", now.Format(snapshotStamp), schema))
	// Under another name until it is whole: a half-written file must never
	// pass for a copy.
	tmp := final + ".part"
	os.Remove(tmp)
	if _, err := db.Exec(`VACUUM INTO ?`, tmp); err != nil {
		os.Remove(tmp)
		return false, fmt.Errorf("copy: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return false, err
	}
	m.pruneSnapshots(id)
	return true, nil
}

// pruneSnapshots keeps the newest snapshotDaily copies, then the newest of
// each earlier week for snapshotWeekly weeks, and removes the rest.
func (m *storyManager) pruneSnapshots(id string) {
	all := m.snapshots(id)
	weeks := map[[2]int]bool{}
	for i, s := range all {
		if i < snapshotDaily {
			continue
		}
		y, w := s.at.ISOWeek()
		key := [2]int{y, w}
		if !weeks[key] && len(weeks) < snapshotWeekly {
			weeks[key] = true
			continue
		}
		if err := os.Remove(filepath.Join(m.snapshotsDir(id), s.Name)); err != nil {
			log.Printf("snapshots %s: %v", id, err)
		}
	}
}

// snapshotAll looks at every story once.
func (m *storyManager) snapshotAll() {
	rows, err := m.reg.Query(`SELECT id FROM stories`)
	if err != nil {
		log.Printf("snapshots: %v", err)
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("snapshots: %v", err)
	}
	rows.Close()
	for _, id := range ids {
		if made, err := m.snapshotStory(id); err != nil {
			log.Printf("snapshot of story %s failed: %v", id, err)
		} else if made {
			log.Printf("story %s: daily copy saved", id)
		}
	}
}

// snapshotLoop looks every hour until ctx ends: a server that is only on in
// the evenings still gets its copy that evening. AUTO_BACKUPS=off turns the
// copies off.
func (m *storyManager) snapshotLoop(ctx context.Context) {
	switch os.Getenv("AUTO_BACKUPS") {
	case "off", "0", "false", "no":
		log.Print("daily copies are off (AUTO_BACKUPS)")
		return
	}
	wait := 2 * time.Minute // not in the way of starting up
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		m.snapshotAll()
		wait = snapshotLookGap
	}
}

// restoreSnapshot brings a copy back as a new story beside the one it was
// taken of: its database as it was, and the pictures the story has now.
func (m *storyManager) restoreSnapshot(id, name string) (storyInfo, error) {
	src, _, err := m.get(id)
	if err != nil {
		return storyInfo{}, err
	}
	var snap *storySnapshot
	for _, s := range m.snapshots(id) {
		if s.Name == name {
			snap = &s
			break
		}
	}
	if snap == nil {
		return storyInfo{}, userError("that copy is gone")
	}
	snapPath := filepath.Join(m.snapshotsDir(id), snap.Name)
	if err := checkImportedDB(snapPath); err != nil {
		return storyInfo{}, err
	}

	newID, err := newStoryID()
	if err != nil {
		return storyInfo{}, err
	}
	ok := false
	defer func() {
		if !ok {
			m.removeStory(newID)
		}
	}()
	if err := os.MkdirAll(m.storyDir(newID), 0o755); err != nil {
		return storyInfo{}, err
	}
	if err := copyFile(snapPath, m.dbPathFor(newID)); err != nil {
		return storyInfo{}, err
	}
	if err := copyTree(m.uploadsFor(id), m.uploadsFor(newID)); err != nil {
		return storyInfo{}, err
	}
	title := fmt.Sprintf("%s (%s)", src.Name, snap.at.Format("2006-01-02"))
	if r := []rune(title); len(r) > maxStoryName {
		keep := maxStoryName - len([]rune(snap.at.Format(" (2006-01-02)")))
		title = string([]rune(src.Name)[:keep]) + snap.at.Format(" (2006-01-02)")
	}
	var icon string
	m.reg.QueryRow(`SELECT COALESCE(icon, '') FROM stories WHERE id = ?`, id).Scan(&icon)
	if _, err := m.reg.Exec(`INSERT INTO stories (id, name, icon) VALUES (?, ?, ?)`, newID, title, icon); err != nil {
		return storyInfo{}, err
	}
	_, err = m.openStory(newID, false) // upgrades an older copy to this version's schema
	if err != nil {
		return storyInfo{}, err
	}
	ok = true
	si, _, err := m.get(newID)
	return si, err
}

// copyTree copies a folder's files into another, folders and all. A missing
// source is an empty one.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if err := copyFile(p, filepath.Join(dst, rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
}

func (m *storyManager) snapshotsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, _, err := m.get(id); err != nil {
		m.fail(w, err)
		return
	}
	writeJSON(w, m.snapshots(id))
}

func (m *storyManager) restoreSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	si, err := m.restoreSnapshot(r.PathValue("id"), r.PathValue("name"))
	if err != nil {
		var ue userError
		if errors.Is(err, errNoStory) || errors.As(err, &ue) {
			m.fail(w, err)
		} else {
			log.Printf("restore copy: %v", err)
			http.Error(w, "failed to bring that copy back", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, si)
}
