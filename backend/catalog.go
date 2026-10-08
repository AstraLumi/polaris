package main

import (
	"database/sql"
	"log"
	"net/http"
)

type lookupOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type classOption struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IconPath string `json:"icon_path"`
}

// listClassesHandler is the class catalog: like the other simple lookups,
// plus each class's icon.
func listClassesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT id, name, COALESCE(icon_path, '') FROM classes ORDER BY name COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load classes", http.StatusInternalServerError)
			log.Printf("list classes: %v", err)
			return
		}
		defer rows.Close()

		results := []classOption{}
		for rows.Next() {
			var o classOption
			if err := rows.Scan(&o.ID, &o.Name, &o.IconPath); err != nil {
				http.Error(w, "failed to read classes", http.StatusInternalServerError)
				return
			}
			o.IconPath = uploadURL(o.IconPath)
			results = append(results, o)
		}
		writeJSON(w, results)
	}
}

func listSimpleLookupHandler(db *sql.DB, table string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT id, name FROM ` + table + ` ORDER BY name COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load "+table, http.StatusInternalServerError)
			log.Printf("list %s: %v", table, err)
			return
		}
		defer rows.Close()

		results := []lookupOption{}
		for rows.Next() {
			var o lookupOption
			if err := rows.Scan(&o.ID, &o.Name); err != nil {
				http.Error(w, "failed to read "+table, http.StatusInternalServerError)
				return
			}
			results = append(results, o)
		}
		writeJSON(w, results)
	}
}

// listSubclassesHandler returns every subclass along with the names of
// every class it's restricted to (empty means "not restricted — shows up
// for any class"). This is what the character form's Subclass combo box
// filters against.
func listSubclassesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT id, name FROM subclasses ORDER BY name COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load subclasses", http.StatusInternalServerError)
			log.Printf("list subclasses: %v", err)
			return
		}
		defer rows.Close()

		type subclassOption struct {
			ID         int64    `json:"id"`
			Name       string   `json:"name"`
			ClassNames []string `json:"class_names"`
		}
		results := []subclassOption{}
		for rows.Next() {
			var o subclassOption
			if err := rows.Scan(&o.ID, &o.Name); err != nil {
				http.Error(w, "failed to read subclasses", http.StatusInternalServerError)
				return
			}
			o.ClassNames = []string{}
			results = append(results, o)
		}
		rows.Close()

		classRows, err := db.Query(`
			SELECT sc.subclass_id, c.name
			FROM subclass_classes sc
			JOIN classes c ON c.id = sc.class_id
			ORDER BY c.name COLLATE NOCASE
		`)
		if err != nil {
			http.Error(w, "failed to load subclass classes", http.StatusInternalServerError)
			log.Printf("list subclass_classes: %v", err)
			return
		}
		defer classRows.Close()

		byID := map[int64]*subclassOption{}
		for i := range results {
			byID[results[i].ID] = &results[i]
		}
		for classRows.Next() {
			var subclassID int64
			var className string
			if err := classRows.Scan(&subclassID, &className); err != nil {
				http.Error(w, "failed to read subclass classes", http.StatusInternalServerError)
				return
			}
			if o, ok := byID[subclassID]; ok {
				o.ClassNames = append(o.ClassNames, className)
			}
		}

		writeJSON(w, results)
	}
}

// listClassScopedLookupHandler powers specializations, which stay on the
// simpler single-required-class model.
func listClassScopedLookupHandler(db *sql.DB, table string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT t.id, t.name, COALESCE(c.name, '')
			FROM ` + table + ` t
			LEFT JOIN classes c ON c.id = t.class_id
			ORDER BY t.name COLLATE NOCASE
		`)
		if err != nil {
			http.Error(w, "failed to load "+table, http.StatusInternalServerError)
			log.Printf("list %s: %v", table, err)
			return
		}
		defer rows.Close()

		type option struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			ClassName string `json:"class_name"`
		}
		results := []option{}
		for rows.Next() {
			var o option
			if err := rows.Scan(&o.ID, &o.Name, &o.ClassName); err != nil {
				http.Error(w, "failed to read "+table, http.StatusInternalServerError)
				return
			}
			results = append(results, o)
		}
		writeJSON(w, results)
	}
}
