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
// (characters, events, the map, calendar). The stat system goes along too, as
// the bonuses and spells are written against its stats.
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
	// Columns whose text can hold links by id (wikiLinkColumns). Ids are
	// kept, so links to copied rows still hold; links to anything else are
	// turned back into names (linksForCopy).
	linkCols []string
	// replace: a new story already has rows here (its default stat system);
	// they are dropped so the copy takes their place.
	replace bool
}

// Copied, in dependency order (a table comes after every table it references).
var assetTables = []assetTable{
	// The stat system: which stats there are and their formulas. Bonuses
	// below and every character sheet are keyed by these stats.
	{name: "stat_defs", replace: true},
	{name: "stat_settings", replace: true},
	{name: "classes", fileCols: []string{"icon_path"}},
	{name: "subclasses"},
	{name: "subclass_classes"},
	{name: "specializations", nullCols: []string{"kingdom_id"}},
	{name: "races", nullCols: []string{"kingdom_id"}},
	{name: "body_types"},
	// Modifiers for classes, subclasses, specializations, races, body types —
	// and, once they exist, items — all point at asset tables, so they travel
	// with them.
	{name: "stat_modifiers"},
	{name: "spells", nullCols: []string{"origin_location_id"}, fileCols: []string{"icon_path"}, linkCols: []string{"description"}},
	{name: "gear", fileCols: []string{"icon_path"}, linkCols: []string{"description"}},
}

// copiedKinds are the kinds of wiki article whose rows copyAssets copies,
// ids and all.
func copiedKinds() map[string]bool {
	out := map[string]bool{}
	for _, wt := range wikiTypes {
		for _, a := range assetTables {
			if a.name == wt.Table {
				out[wt.Key] = true
			}
		}
	}
	return out
}

// Belong to one story's own world; never copied.
var storyOnlyTables = []string{
	"characters", "character_versions", "character_story",
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
	// The bestiary lives in the world (creatures are native to map regions).
	"creatures", "creature_links", "creature_hexes",
	// A region's temperatures per season.
	"location_climate",
	// The planet's painted cells.
	"planet_cells", "creature_cells",
	// The solar system round the world.
	"celestial_bodies", "body_cells", "solar_systems",
	// The planet's layers.
	"map_features", "feature_cells", "cell_biomes", "cell_relief",
	// The book itself: each book's settings, each chapter's text and its
	// saved versions (editor.go).
	"book_settings", "chapter_texts", "chapter_text_versions",
	// What each chapter's text names, and its lines (chapterindex.go).
	"chapter_links", "chapter_lines",
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
// link between assets (subclass → class, spell source, modifiers, a [[link]]
// by id in a description) stays valid. src is the source story's open
// database, for the names of what the copied texts link to.
func copyAssets(dst *sql.DB, src wikiQuerier, srcDBPath, srcUploads, dstUploads string) error {
	ctx := context.Background()
	links, err := loadWikiLinkIndex(src)
	if err != nil {
		return fmt.Errorf("index the source's wiki: %w", err)
	}
	copied := copiedKinds()
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
		if t.replace {
			if _, err := conn.ExecContext(ctx, `DELETE FROM main.`+quoteIdent(t.name)); err != nil {
				return rollback(err)
			}
		}
		q := fmt.Sprintf(`INSERT INTO main.%s (%s) SELECT %s FROM src.%s`,
			quoteIdent(t.name), strings.Join(ins, ", "), strings.Join(sel, ", "), quoteIdent(t.name))
		if _, err := conn.ExecContext(ctx, q); err != nil {
			return rollback(fmt.Errorf("copy %s: %w", t.name, err))
		}
		for _, lc := range t.linkCols {
			if err := relinkCopied(ctx, conn, t.name, lc, links, copied); err != nil {
				return rollback(fmt.Errorf("copy %s.%s: %w", t.name, lc, err))
			}
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

// relinkCopied runs linksForCopy over a copied text column.
func relinkCopied(ctx context.Context, conn *sql.Conn, table, col string, x *wikiLinkIndex, copied map[string]bool) error {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, %s FROM main.%s WHERE %s LIKE '%%[[%%'`,
		quoteIdent(col), quoteIdent(table), quoteIdent(col)))
	if err != nil {
		return err
	}
	changed := map[int64]string{}
	for rows.Next() {
		var id int64
		var s string
		if err := rows.Scan(&id, &s); err != nil {
			rows.Close()
			return err
		}
		if out := linksForCopy(s, x, copied); out != s {
			changed[id] = out
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for id, s := range changed {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`UPDATE main.%s SET %s = ? WHERE id = ?`, quoteIdent(table), quoteIdent(col)), s, id); err != nil {
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
