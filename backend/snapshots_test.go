package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// A daily copy is made when the story changed since the last one, at most
// once a day; the newest 7 are kept, then one a week for 4 weeks.
func TestDailyCopies(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Kept")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO classes (name) VALUES ('Keeper')`)

	// The clock starts well in the past, so a file written now is always
	// newer than a copy.
	clock := time.Now().UTC().Add(-60 * 24 * time.Hour).Truncate(time.Second)
	defer func(old func() time.Time) { snapshotNow = old }(snapshotNow)
	snapshotNow = func() time.Time { return clock }
	take := func() bool {
		t.Helper()
		made, err := mgr.snapshotStory(id)
		if err != nil {
			t.Fatal(err)
		}
		return made
	}
	touch := func(at time.Time) {
		t.Helper()
		if err := os.Chtimes(mgr.dbPathFor(id), at, at); err != nil {
			t.Fatal(err)
		}
	}

	if !take() {
		t.Fatal("the first copy is made")
	}
	if take() {
		t.Error("not twice within a day")
	}
	clock = clock.Add(25 * time.Hour)
	touch(clock.Add(-30 * time.Hour)) // last written before the copy
	if take() {
		t.Error("not when the story hasn't changed since")
	}
	touch(clock.Add(-time.Hour))
	if !take() {
		t.Error("a day later, changed: a new copy")
	}
	if left, _ := os.ReadDir(mgr.snapshotsDir(id)); len(left) != 2 {
		t.Fatalf("two copies, and no half-written file: %v", left)
	}

	// Forty more days, a change and a copy each day.
	for range 40 {
		clock = clock.Add(24 * time.Hour)
		touch(clock.Add(-time.Hour))
		if !take() {
			t.Fatal("a copy each day")
		}
	}
	have := mgr.snapshots(id)
	if len(have) != snapshotDaily+snapshotWeekly {
		t.Fatalf("kept %d copies, want %d", len(have), snapshotDaily+snapshotWeekly)
	}
	for i := 1; i < snapshotDaily; i++ {
		if gap := have[i-1].at.Sub(have[i].at); gap != 24*time.Hour {
			t.Errorf("the newest seven are a day apart: %v", gap)
		}
	}
	weeks := map[[2]int]bool{}
	for _, s := range have[snapshotDaily:] {
		y, w := s.at.ISOWeek()
		if weeks[[2]int{y, w}] {
			t.Errorf("one a week beyond the first seven: %s", s.Name)
		}
		weeks[[2]int{y, w}] = true
	}
	if have[0].Schema != currentSchemaVersion || have[0].Size == 0 {
		t.Errorf("a copy knows its schema and size: %+v", have[0])
	}

	// A story that was never opened has nothing to copy, and isn't opened
	// for it.
	if _, err := mgr.reg.Exec(`INSERT INTO stories (id, name) VALUES ('00000000000000aa', 'Unopened')`); err != nil {
		t.Fatal(err)
	}
	if made, err := mgr.snapshotStory("00000000000000aa"); made || err != nil {
		t.Errorf("an unopened story: %v %v", made, err)
	}
	mgr.snapshotAll() // every story, without failing on that one
}

// A copy comes back as a new story beside the present one, as it was then.
func TestBringACopyBack(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Saga")
	addClass(t, h, id, "Before")
	if made, err := mgr.snapshotStory(id); !made || err != nil {
		t.Fatalf("copy: %v %v", made, err)
	}
	addClass(t, h, id, "After")

	rec := do(t, h, "GET", "/api/stories/"+id+"/snapshots", nil, "")
	var list []storySnapshot
	if json.Unmarshal(rec.Body.Bytes(), &list); rec.Code != 200 || len(list) != 1 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, "POST", "/api/stories/"+id+"/snapshots/"+list[0].Name+"/restore", nil, "")
	var back storyInfo
	if json.Unmarshal(rec.Body.Bytes(), &back); rec.Code != 200 || back.ID == "" || back.ID == id {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(back.Name, "Saga (") {
		t.Errorf("named after the story and the day: %q", back.Name)
	}
	then := do(t, h, "GET", "/api/s/"+back.ID+"/classes", nil, "").Body.String()
	now := do(t, h, "GET", "/api/s/"+id+"/classes", nil, "").Body.String()
	if !strings.Contains(then, "Before") || strings.Contains(then, "After") {
		t.Errorf("the copy is the story as it was: %s", then)
	}
	if !strings.Contains(now, "After") {
		t.Errorf("the story itself is untouched: %s", now)
	}

	for _, bad := range []string{"snap-20000101-000000-s46.db", "..%2Fstory.db", "story.db"} {
		if rec := do(t, h, "POST", "/api/stories/"+id+"/snapshots/"+bad+"/restore", nil, ""); rec.Code != 400 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
	if rec := do(t, h, "GET", "/api/stories/0000000000000000/snapshots", nil, ""); rec.Code != 404 {
		t.Errorf("no such story: %d", rec.Code)
	}
}
