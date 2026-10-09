package main

import (
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// The frontend's compiled Vite output is copied into ./static at Docker
// build time (see the Dockerfile) and embedded directly into the binary,
// so the final container has nothing to serve but a single executable.
//
//go:embed all:static
var embeddedFrontend embed.FS

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Everything a story owns lives under these two roots (one folder per
	// story in each). DATA_DIR follows DB_PATH's folder so the existing
	// docker-compose volumes keep working.
	dataDir := getEnv("DATA_DIR", filepath.Dir(getEnv("DB_PATH", "/data/polaris.db")))
	uploadsRoot := getEnv("UPLOADS_DIR", "/app/uploads")

	mgr, err := newStoryManager(dataDir, uploadsRoot)
	if err != nil {
		log.Fatalf("failed to start: %v", err)
	}
	defer mgr.Close()

	staticFS, err := fs.Sub(embeddedFrontend, "static")
	if err != nil {
		log.Fatalf("failed to load embedded frontend assets: %v", err)
	}

	port := getEnv("PORT", "8091")
	log.Printf("Polaris listening on :%s (data: %s, uploads: %s)", port, dataDir, uploadsRoot)
	log.Fatal(http.ListenAndServe(":"+port, newRouter(mgr, staticFS)))
}

// newRouter is the whole server: the frontend, the story registry API, and
// every story's own API under /api/s/{id}/.
func newRouter(mgr *storyManager, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", spaHandler(staticFS))
	mgr.registerRoutes(mux)
	return mux
}

// storyRoutes is one story's API. Every handler is bound to that story's
// database and uploads folder, so a story can't see another's data. Paths
// here are the plain /api/... ones; the manager strips /api/s/{id} before
// dispatching (see stories.go).
func storyRoutes(db *sql.DB, uploadsDir string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", healthHandler(db))

	// Characters (identity + grid).
	mux.HandleFunc("GET /api/characters", listCharactersHandler(db))
	mux.HandleFunc("POST /api/characters", createCharacterHandler(db, uploadsDir))
	mux.HandleFunc("DELETE /api/characters", deleteCharactersHandler(db, uploadsDir))
	mux.HandleFunc("GET /api/characters/{id}/current", getCurrentVersionHandler(db))

	// Lookup catalogs, for the combo-box fields on the character forms.
	mux.HandleFunc("GET /api/classes", listClassesHandler(db))
	mux.HandleFunc("GET /api/subclasses", listSubclassesHandler(db))
	mux.HandleFunc("GET /api/specializations", listClassScopedLookupHandler(db, "specializations"))
	mux.HandleFunc("GET /api/races", listSimpleLookupHandler(db, "races"))
	mux.HandleFunc("GET /api/body-types", listSimpleLookupHandler(db, "body_types"))

	// Character Assets: full CRUD + stat modifiers for every asset type
	// except Spells.
	mux.HandleFunc("GET /api/classes/{id}", getSimpleAssetHandler(db, "classes", "class"))
	mux.HandleFunc("POST /api/classes", createSimpleAssetHandler(db, "classes", "class"))
	mux.HandleFunc("PUT /api/classes/{id}", updateSimpleAssetHandler(db, "classes", "class"))
	mux.HandleFunc("DELETE /api/classes/{id}", deleteSimpleAssetHandler(db, uploadsDir, "classes", "class"))
	mux.HandleFunc("PUT /api/classes/{id}/icon", setClassIconHandler(db, uploadsDir))
	mux.HandleFunc("DELETE /api/classes/{id}/icon", clearClassIconHandler(db, uploadsDir))

	mux.HandleFunc("GET /api/races/{id}", getSimpleAssetHandler(db, "races", "race"))
	mux.HandleFunc("POST /api/races", createSimpleAssetHandler(db, "races", "race"))
	mux.HandleFunc("PUT /api/races/{id}", updateSimpleAssetHandler(db, "races", "race"))
	mux.HandleFunc("DELETE /api/races/{id}", deleteSimpleAssetHandler(db, uploadsDir, "races", "race"))

	mux.HandleFunc("GET /api/body-types/{id}", getSimpleAssetHandler(db, "body_types", "body_type"))
	mux.HandleFunc("POST /api/body-types", createSimpleAssetHandler(db, "body_types", "body_type"))
	mux.HandleFunc("PUT /api/body-types/{id}", updateSimpleAssetHandler(db, "body_types", "body_type"))
	mux.HandleFunc("DELETE /api/body-types/{id}", deleteSimpleAssetHandler(db, uploadsDir, "body_types", "body_type"))

	mux.HandleFunc("GET /api/specializations/{id}", getSpecializationHandler(db))
	mux.HandleFunc("POST /api/specializations", createSpecializationHandler(db))
	mux.HandleFunc("PUT /api/specializations/{id}", updateSpecializationHandler(db))
	mux.HandleFunc("DELETE /api/specializations/{id}", deleteSpecializationHandler(db, uploadsDir))

	mux.HandleFunc("GET /api/subclasses/{id}", getSubclassHandler(db))
	mux.HandleFunc("POST /api/subclasses", createSubclassHandler(db))
	mux.HandleFunc("PUT /api/subclasses/{id}", updateSubclassHandler(db))
	mux.HandleFunc("DELETE /api/subclasses/{id}", deleteSubclassHandler(db))

	// Stateless stat computation for the Edit page's live preview — see
	// preview.go for why this exists instead of duplicating formulas in JS.
	mux.HandleFunc("POST /api/compute-preview", computePreviewHandler(db))

	// Spells (created on the Character Assets page, then assigned to a
	// character version from its Spells tab).
	mux.HandleFunc("GET /api/spells", listSpellsHandler(db))
	mux.HandleFunc("GET /api/spells/{id}", getSpellHandler(db))
	mux.HandleFunc("POST /api/spells", createSpellHandler(db, uploadsDir))
	mux.HandleFunc("PUT /api/spells/{id}", updateSpellHandler(db, uploadsDir))
	mux.HandleFunc("DELETE /api/spells/{id}", deleteSpellHandler(db, uploadsDir))

	// Gear: created on the Character Assets page, then equipped from a
	// character version's Gear tab.
	mux.HandleFunc("GET /api/gear", listGearHandler(db))
	mux.HandleFunc("GET /api/gear/{id}", getGearHandler(db))
	mux.HandleFunc("POST /api/gear", createGearHandler(db, uploadsDir))
	mux.HandleFunc("PUT /api/gear/{id}", updateGearHandler(db, uploadsDir))
	mux.HandleFunc("DELETE /api/gear/{id}", deleteGearHandler(db, uploadsDir))

	// Locations: the lookup list still feeds a spell's Origin combo box,
	// while the Map page owns creating and placing them.
	mux.HandleFunc("GET /api/locations", listSimpleLookupHandler(db, "locations"))

	mux.HandleFunc("GET /api/location-options", listLocationOptionsHandler(db))

	// Calendar and other app-wide settings.
	mux.HandleFunc("GET /api/settings", getSettingsHandler(db))
	mux.HandleFunc("PUT /api/settings", updateSettingsHandler(db))

	// Events: single-date happenings that feed the timeline. Tags are the
	// one linking mechanism; character-options feeds the "involved people"
	// picker.
	mux.HandleFunc("GET /api/events", listEventsHandler(db))
	mux.HandleFunc("POST /api/events", createEventHandler(db, uploadsDir))
	mux.HandleFunc("POST /api/events/from-source", createEventFromSourceHandler(db))
	mux.HandleFunc("GET /api/events/{id}", getEventHandler(db))
	mux.HandleFunc("PUT /api/events/{id}", updateEventHandler(db, uploadsDir))
	mux.HandleFunc("DELETE /api/events/{id}", deleteEventHandler(db, uploadsDir))
	mux.HandleFunc("GET /api/event-tags", listEventTagsHandler(db))
	mux.HandleFunc("GET /api/character-options", listCharacterOptionsHandler(db))

	// Timeline: every dated event, birth and founding on one number line.
	mux.HandleFunc("GET /api/timeline", timelineHandler(db))

	// Wiki: an article for everything in the story, with the user's own
	// text layered on top (see wiki.go).
	mux.HandleFunc("GET /api/wiki", listWikiHandler(db))
	mux.HandleFunc("GET /api/wiki/{type}/{id}", getWikiHandler(db))
	mux.HandleFunc("PUT /api/wiki/{type}/{id}", saveWikiHandler(db))

	// Home dashboard.
	mux.HandleFunc("GET /api/home", homeHandler(db))

	// Map: every location and hex in one payload, location CRUD, and
	// batched hex painting.
	mux.HandleFunc("GET /api/map", getMapHandler(db))
	mux.HandleFunc("POST /api/map/locations", createMapLocationHandler(db))
	mux.HandleFunc("PUT /api/map/locations/{id}", updateMapLocationHandler(db))
	mux.HandleFunc("DELETE /api/map/locations/{id}", deleteMapLocationHandler(db, uploadsDir))
	mux.HandleFunc("POST /api/map/paint", paintHexesHandler(db))
	mux.HandleFunc("POST /api/map/erase", eraseHexesHandler(db))

	// Versions (the actual editable/viewable sheets).
	mux.HandleFunc("GET /api/characters/{id}/versions", listVersionsHandler(db))
	mux.HandleFunc("POST /api/characters/{id}/versions", createVersionHandler(db))
	mux.HandleFunc("GET /api/versions/{id}", getVersionHandler(db))
	mux.HandleFunc("PUT /api/versions/{id}", updateVersionHandler(db, uploadsDir))
	mux.HandleFunc("DELETE /api/versions/{id}", deleteVersionHandler(db, uploadsDir))
	mux.HandleFunc("POST /api/versions/{id}/set-current", setCurrentVersionHandler(db))

	return mux
}

// spaHandler serves embedded static files normally, but falls back to
// index.html for any path that isn't a real file — so a browser refresh on
// a vue-router route like /characters/3/versions still works instead of
// 404ing.
func spaHandler(staticFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(staticFS, path); err != nil {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fileServer.ServeHTTP(w, r2)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

// healthHandler confirms the database is reachable and reports the schema
// version, so /api/health doubles as a quick sanity check after deploys.
func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var version string
		err := db.QueryRow(
			"SELECT value FROM schema_meta WHERE key = 'schema_version'",
		).Scan(&version)

		status := "ok"
		if err != nil {
			status = "db_error"
			version = ""
		}

		writeJSON(w, map[string]string{
			"status":         status,
			"schema_version": version,
		})
	}
}
