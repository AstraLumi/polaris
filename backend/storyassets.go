package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// "New story, copying another story's Character Assets" copies the reusable
// library (classes, subclasses, specializations, races, body types, spells and
// the stat modifiers attached to them) and nothing about the world itself
// (characters, events, the map, calendar).
//
// Adding a table to the schema? Put it in exactly one of the two lists below —
// TestEveryTableIsClassified fails until you do. The upcoming Gear tab's
// tables belong in assetTables (parents before children), and its icons /
// pictures go in fileCols so the copied rows keep their images.

type assetTable struct {
	name string
	// Columns that point at things that are NOT copied (e.g. a spell's origin
	// location lives on the map); they are set to NULL in the copy.
	nullCols []string
	// Columns holding an uploaded file's path inside the uploads folder; the
	// files are copied along with the rows.
	fileCols []string
}

// Copied, in dependency order (a table comes after every table it references).
var assetTables = []assetTable{
	{name: "classes", fileCols: []string{"icon_path"}},
	{name: "subclasses"},
	{name: "subclass_classes"},
	{name: "specializations"},
	{name: "races"},
	{name: "body_types"},
	// Modifiers for classes, subclasses, specializations, races, body types —
	// and, once they exist, items — all point at asset tables, so they travel
	// with them.
	{name: "stat_modifiers"},
	{name: "spells", nullCols: []string{"origin_location_id"}, fileCols: []string{"icon_path"}},
	{name: "gear", fileCols: []string{"icon_path"}},
}

// Belong to one story's own world; never copied.
var storyOnlyTables = []string{
	"characters", "character_versions", "character_story", "character_build",
	"character_special_bases", "character_spells", "character_gear",
	"events", "event_tags", "event_characters",
	"locations", "location_hexes",
	"app_settings",
	// The wiki's extra text belongs to the story, not the reusable library.
	"wiki_entries", "wiki_sections", "wiki_infobox",
	// Who is related to whom, the story's factions, and its chapters.
	"character_relations", "factions", "faction_members", "volumes", "chapters",
	// The wiki's free-form articles, and the tags on characters.
	"lore_articles", "character_tags", "wiki_gallery",
}

func isNullCol(t assetTable, col string) bool {
	for _, c := range t.nullCols {
		if c == col {
			return true
		}
	}
	return false
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// copyAssets fills the (new, empty) database dst with the asset library found
// in the story database at srcDBPath, then copies the uploaded files those rows
// refer to from srcUploads to dstUploads. Row ids are preserved, so every
// link between assets (subclass → class, spell source, modifiers) stays valid.
func copyAssets(dst *sql.DB, srcDBPath, srcUploads, dstUploads string) error {
	ctx := context.Background()
	conn, err := dst.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS src`, srcDBPath); err != nil {
		return fmt.Errorf("attach source: %w", err)
	}
	defer conn.ExecContext(ctx, `DETACH DATABASE src`)

	if _, err := conn.ExecContext(ctx, `BEGIN`); err != nil {
		return err
	}
	rollback := func(err error) error {
		conn.ExecContext(ctx, `ROLLBACK`)
		return err
	}

	var files []string
	for _, t := range assetTables {
		rows, err := conn.QueryContext(ctx, `PRAGMA main.table_info(`+quoteIdent(t.name)+`)`)
		if err != nil {
			return rollback(err)
		}
		var cols []string
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				rows.Close()
				return rollback(err)
			}
			cols = append(cols, name)
		}
		rows.Close()
		if len(cols) == 0 {
			return rollback(fmt.Errorf("table %s missing", t.name))
		}
		var ins, sel []string
		for _, c := range cols {
			ins = append(ins, quoteIdent(c))
			if isNullCol(t, c) {
				sel = append(sel, "NULL")
			} else {
				sel = append(sel, quoteIdent(c))
			}
		}
		q := fmt.Sprintf(`INSERT INTO main.%s (%s) SELECT %s FROM src.%s`,
			quoteIdent(t.name), strings.Join(ins, ", "), strings.Join(sel, ", "), quoteIdent(t.name))
		if _, err := conn.ExecContext(ctx, q); err != nil {
			return rollback(fmt.Errorf("copy %s: %w", t.name, err))
		}
		for _, fc := range t.fileCols {
			fr, err := conn.QueryContext(ctx, fmt.Sprintf(
				`SELECT %s FROM main.%s WHERE %s IS NOT NULL AND %s != ''`,
				quoteIdent(fc), quoteIdent(t.name), quoteIdent(fc), quoteIdent(fc)))
			if err != nil {
				return rollback(err)
			}
			for fr.Next() {
				var p string
				if err := fr.Scan(&p); err == nil && !isBuiltinIcon(p) {
					files = append(files, p)
				}
			}
			fr.Close()
		}
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return rollback(err)
	}

	for _, rel := range files {
		clean, ok := safeRel(rel)
		if !ok {
			continue
		}
		if err := copyFile(filepath.Join(srcUploads, filepath.FromSlash(clean)), filepath.Join(dstUploads, filepath.FromSlash(clean))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
