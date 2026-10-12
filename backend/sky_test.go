package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func skyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// A story's own sky: checked, stored with its uploads, and kept when the map
// settings are saved; deleting it goes back to the default.
func TestSky(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Sky"})
	base := "/api/s/" + s.ID
	upload := func(data []byte) (int, mapSettings) {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, _ := mw.CreateFormFile("file", "stars.png")
		fw.Write(data)
		mw.Close()
		rec := do(t, h, "PUT", base+"/planet/sky", &body, mw.FormDataContentType())
		var out mapSettings
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if code, _ := upload(skyPNG(t, 1000, 1000)); code != 400 {
		t.Errorf("a square sky: %d", code)
	}
	if code, _ := upload([]byte("not an image")); code != 400 {
		t.Errorf("not an image: %d", code)
	}
	code, got := upload(skyPNG(t, 1024, 512))
	if code != 200 || got.Planet.Sky == "" || !strings.Contains(got.SkyURL, "/uploads/") {
		t.Fatalf("upload: %d %+v", code, got)
	}
	app, _ := mgr.open(s.ID)
	file := filepath.Join(app.uploadsDir, filepath.FromSlash(got.Planet.Sky))
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("the sky is stored with the story's uploads: %v", err)
	}
	// Saving the map settings keeps it.
	saved := putMap(t, h, base, nil)
	if saved.Planet.Sky != got.Planet.Sky {
		t.Errorf("map settings dropped the sky: %+v", saved.Planet)
	}
	rec := do(t, h, "DELETE", base+"/planet/sky", nil, "")
	var back mapSettings
	json.Unmarshal(rec.Body.Bytes(), &back)
	if rec.Code != 200 || back.Planet.Sky != "" || back.SkyURL != "" {
		t.Errorf("back to the default: %d %+v", rec.Code, back)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("the old sky picture is removed")
	}
}
