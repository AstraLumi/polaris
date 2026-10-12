package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // registers the decoders image.DecodeConfig reads
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// The sky behind the planet in the 3D map: by default Polaris's own
// (frontend/public/sky/default-sky.jpg, drawn by cmd/skygen, which also
// explains the format), or a picture uploaded for the story. It must be an
// equirectangular image twice as wide as it is tall, the top edge the sky
// above the north pole, so it folds onto the sky exactly.

const (
	maxSkyBytes = 40 << 20
	maxSkyWidth = 8192 // the messages below say so
	minSkyWidth = 512
)

// checkSky reads an uploaded sky's size and says what's wrong with it.
func checkSky(data []byte) string {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return "the sky must be a JPEG or PNG image"
	}
	if math.Abs(float64(cfg.Width)/float64(cfg.Height)-2) > 0.02 {
		return "the sky must be twice as wide as it is tall (for example 4096 × 2048)"
	}
	if cfg.Width > maxSkyWidth || cfg.Width < minSkyWidth {
		return "the sky must be between 512 and 8192 pixels wide"
	}
	return ""
}

// saveSky stores the planet settings with a new sky path ("" = the default).
func saveSky(db *sql.DB, rel string) error {
	p := loadMapSettings(db).Planet
	p.Sky = rel
	raw, _ := json.Marshal(p)
	_, err := db.Exec(`INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, settingPlanet, string(raw))
	return err
}

func uploadSkyHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxSkyBytes+(1<<20))
		if err := r.ParseMultipartForm(maxFormMemory); err != nil {
			http.Error(w, "the sky picture is too large", http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "choose a picture for the sky", http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxSkyBytes+1))
		if err != nil || len(data) > maxSkyBytes {
			http.Error(w, "the sky picture is too large", http.StatusBadRequest)
			return
		}
		if msg := checkSky(data); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			ext = ".jpg"
			if _, format, _ := image.DecodeConfig(bytes.NewReader(data)); format == "png" {
				ext = ".png"
			}
		}
		rel, err := saveUpload(uploadsDir, "sky", fmt.Sprintf("sky-%d", time.Now().UnixNano()), "sky"+ext, bytes.NewReader(data))
		if err != nil {
			log.Printf("save sky: %v", err)
			http.Error(w, "failed to save the sky", http.StatusInternalServerError)
			return
		}
		old := loadMapSettings(db).Planet.Sky
		if err := saveSky(db, rel); err != nil {
			removePicture(uploadsDir, rel)
			log.Printf("save sky setting: %v", err)
			http.Error(w, "failed to save the sky", http.StatusInternalServerError)
			return
		}
		removePicture(uploadsDir, old)
		writeJSON(w, loadMapSettings(db))
	}
}

// resetSkyHandler goes back to the default sky.
func resetSkyHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		old := loadMapSettings(db).Planet.Sky
		if err := saveSky(db, ""); err != nil {
			http.Error(w, "failed to save the sky", http.StatusInternalServerError)
			return
		}
		removePicture(uploadsDir, old)
		writeJSON(w, loadMapSettings(db))
	}
}
