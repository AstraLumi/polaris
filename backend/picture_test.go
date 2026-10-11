package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Replacing a version's picture gives it a new address (the browser would
// otherwise keep showing the old one from its cache) and removes the old file.
func TestReplacingAPicture(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Pictures"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(app.uploadsDir, filepath.FromSlash(rel)))
		return err == nil
	}
	pathOf := func(vid int64) string {
		var p string
		app.db.QueryRow(`SELECT COALESCE(picture_path, '') FROM character_versions WHERE id = ?`, vid).Scan(&p)
		return p
	}
	save := func(vid int64, data string) {
		t.Helper()
		body, ct := form(t, map[string]string{"name": "Aria", "level": "1"}, "picture", "face.png", []byte(data))
		if rec := do(t, h, "PUT", base+"/versions/"+idStr(vid), body, ct); rec.Code != 200 {
			t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
		}
	}

	body, ct := form(t, map[string]string{"name": "Aria", "level": "1"}, "picture", "face.png", []byte("ONE"))
	var c struct {
		ID        int64 `json:"id"`
		VersionID int64 `json:"version_id"`
	}
	json.Unmarshal(do(t, h, "POST", base+"/characters", body, ct).Body.Bytes(), &c)
	first := pathOf(c.VersionID)

	save(c.VersionID, "TWO")
	second := pathOf(c.VersionID)
	if second == first || !exists(second) || exists(first) {
		t.Fatalf("replacing: %q -> %q (old still there: %v)", first, second, exists(first))
	}

}
