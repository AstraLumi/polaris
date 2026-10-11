# Developing Polaris

Notes for working on the code. For installing and running it, see the
[README](../README.md).

## Layout

- `backend/` Go 1.26+, standard library `net/http` plus SQLite
  (`modernc.org/sqlite`, pure Go, no cgo). It embeds the compiled frontend
  (`backend/static`, produced by the frontend build) and serves both the API
  and the pages.
- `frontend/` Vue 3 + Vite.

## Running it for development

Backend (serves the API on :8091; needs a built frontend in `backend/static`,
or just use the placeholder and run the Vite dev server instead):

    cd backend
    # DATA_DIR defaults to /data, so point it somewhere writable:
    DATA_DIR=./data UPLOADS_DIR=./data/uploads go run .

Frontend with hot reload:

    cd frontend
    npm install
    npm run dev                   # proxies /api and /uploads to the backend on :8091

Production-style build into the Go binary:

    cd frontend && npm ci && npm run build
    cp -r dist/. ../backend/static/
    cd ../backend && go build -o polaris .

## Tests

    cd backend && go test ./...      # includes the multi-story tests
    node frontend/tests/timelineLayout.test.mjs   # timeline geometry, no deps
    node frontend/tests/markdown.test.mjs         # the wiki's markdown renderer
    node frontend/tests/i18n.check.mjs            # every UI string translated in every language

CI (`.github/workflows/ci.yml`) runs the same checks.

## Adding or changing the database

Schema changes are forward-only migrations in `backend/db.go`
(`currentSchemaVersion`, `ensureSchema`, `canMigrate`). Never edit an old
migration; add a new one and a test (see `TestMigrationAddsGearTables`).

## Feature and architecture notes

Characters, Character Assets (classes, subclasses, specializations, races, body
types, gear, spells), Map, Events, Timeline, Wiki, Settings and a Home dashboard.

- Stories: the app holds any number of separate stories. Each has its own
  SQLite file and uploads folder, so characters, map, events, assets and
  calendar never mix. The picker (`/stories`) creates, renames (name + icon),
  deletes (type the name to confirm), exports and imports stories; the last
  one opened is remembered per device and reopened, and the sidebar has a
  Switch story button. A new story can start blank or copy another story's
  Character Assets (`backend/storyassets.go`). Export is a `.zip`
  (`polaris-story.json`, `story.db`, `uploads/`); importing always creates a
  new story, so it doubles as backup/restore.
  - API: `/api/stories` (list/create/import), `/api/stories/{id}` (PUT/DELETE
    with `?confirm=<name>`), `/api/stories/{id}/export`; everything inside a
    story is under `/api/s/{id}/...` and its pictures under
    `/uploads/s/{id}/...`.
  - **Adding a table?** List it in `assetTables` (copied
    with "copy Character Assets") or `storyOnlyTables` in
    `backend/storyassets.go`; `TestEveryTableIsClassified` fails until you do.
    Put its picture/icon columns in `fileCols` so copies keep their images.
- Wiki (`backend/wiki.go`, `frontend/src/views/Wiki*.vue`, `markdown.js`,
  `wiki.js`): nobody creates wiki pages. Every character, location, event,
  class, subclass, specialization, race, body type, spell and piece of gear is
  an article, and its infobox and related lists are read live from the source
  tables. Only the extra text is stored (`wiki_entries`: introduction plus the
  fixed markdown fields; `wiki_sections`, `wiki_infobox`: the user's own
  sections and rows). The entry points at one of ten tables, so there is no
  foreign key; instead `wikiSchemaStatements` creates an `AFTER DELETE` trigger
  on each source table that removes its entry, so deleting a source from any
  path deletes its wiki text. Events that only carry a birth or founding are
  not articles. `[[Name]]`, `[[Name|text]]` and `[[type:Name]]` link articles;
  the Go (`resolveWikiLink`) and JS (`makeLinkResolver`) resolvers must agree.
  Markdown is rendered by our own small escaping renderer (no dependency, no raw
  HTML); `node frontend/tests/markdown.test.mjs` covers it. Adding an article
  kind: a `wikiTypeDef`, its facts in `loadSourceFacts`, its labels in
  `wiki.js`, and a migration creating its trigger.
  Lore articles (`backend/lore.go`, schema 17) are the one kind with no
  source: `lore_articles` holds only a name and picture, created from the
  wiki home ("New article"); the text is ordinary wiki text (introduction,
  trivia, sections, infobox) and their Related list is what they link to.
  A lore article's infobox can hold dated rows (`wiki_infobox.kind = 'date'`,
  schema 18), which put it on the timeline as kind `lore`.
  Hovering any link to an article shows a preview card
  (`components/LinkPreviewLayer.vue`, `ArticleCard.vue`, `articlePreview.js`);
  the tooltip layer leaves those links alone.
  The graph view (`/wiki/graph`, `views/GraphView.vue`) draws every article
  and every connection on a canvas: `backend/wikigraph.go` lists the story's
  own connections (the same facts and related lists the articles show) and
  the text's [[links]] separately, and `graphLayout.js` is the force layout
  (`node frontend/tests/graphLayout.test.mjs`). `?focus=<type>:<id>` opens it
  centred on one article; `&depth=N` keeps only what lies within N
  connections of it and `&types=a,b` starts with only those kinds shown.
- Story time (`backend/storytime.go`, `frontend/src/storyTime.js`, kept in
  step): a character version sits at a point (its chapter in story order, and
  its date); relations start and end at a chapter and/or date, and each
  version shows the ones that hold at its point. The wiki gives a character
  one tab of facts and relations per version (`versions` in the article), the
  written text stays shared; the graph and family tree use every relation.
- Former names: a character's names in other versions are aliases
  (`aliases` in the wiki list), so [[links]] and search still find them, and
  the infobox lists them as "Also known as".
- Kingdoms and factions (`backend/kingdoms.go`): every kingdom (a coloured
  major location) has a faction tied by `factions.kingdom_id`, made and kept
  in step from the map; losing the colour or deleting the kingdom unties it.
- Wiki galleries (`backend/wikigallery.go`, schema 19, `components/WikiGallery.vue`):
  any article can hold captioned pictures (`wiki_gallery`, hanging off the
  article's `wiki_entries` row, so deleting the source removes them). They
  save straight away, apart from the article's text; an entry that only holds
  pictures survives clearing the text. `viewGallery()` opens them in the
  picture viewer with arrows.
- Calendar (`/calendar`, `views/CalendarView.vue`): the timeline's points a
  month at a time, in the story's own calendar (no weeks, so days run in rows
  of seven). `?y=&m=` is the month shown.
- Family tree (`/characters/:id/family`, `views/FamilyTree.vue`): everyone
  joined to a character by parent, sibling, spouse and partner relations
  (`backend/family.go`), laid out in generation rows by `familyLayout.js`
  (`node frontend/tests/familyLayout.test.mjs`). For a parent relation,
  `from_id` is the parent.
  `components/WikiLinkPanel.vue` is the shared "Link to an article" picker
  (the Markdown editor and the infobox value rows use it).
- Look and feel: every page shares one layout (`.page`), one set of form
  controls, one modal style and one set of colour variables, all in
  `frontend/src/style.css`. Colour themes (Polaris default, Obsidian,
  Verdant, Crimson, Ashen) are blocks of variables there, listed in
  `frontend/src/theme.js`; the choice is made under Settings and kept per
  device. The logo is `frontend/public/logo.svg` (the in-app copy is
  `components/PolarisLogo.vue`, which follows the theme) and the favicon set
  is in `frontend/public/`.
- Tooltips: one styled tooltip for the whole app
  (`components/TooltipLayer.vue`, mounted in `App.vue`). Give an element a
  plain `title` (translated) and it is shown styled; a trailing shortcut such
  as `(Ctrl+B)` is drawn as keys. Icon-only buttons with an `aria-label` get
  it as their tooltip, and `data-tip-overflow` shows an element's own text
  only while it is cut off by an ellipsis.
- Dates are stored as `DD-MM-YYYY` (year may be negative); a bare year is
  saved as the 1st of the 1st month. The calendar (months per year, days
  per month) is configurable under Settings.
- A character's Born in / Nation link to Map locations (Nation: kingdoms
  only), so renaming a location never orphans them.
- Events are single-date and linked to each other through shared tags (tag a
  war's start and end the same way). They can have an optional Map location,
  involved characters and a picture. Birth/founding "details" events are
  created on demand from a character or location and take their name and date
  from it live; deleting the source deletes them.
- Timeline (`/timeline`): a zigzag line from the earliest to the latest dated
  point, spaced in proportion to time (nodes never sit closer than 48px, so
  same-year events stay separate dots). Gaps over 25 years (adjustable, 0 =
  off) become a captioned coil instead of empty scrolling. Points are events,
  births (each character's *current* version only) and map foundings; the
  latter two are generated live, so they follow renames/redates/deletes.
  Zoom, coil threshold and tag highlight are remembered in the browser.
- There is no year 0: the year before 1 is -1 (year 0 is refused on input).
- Languages: Settings has an interface-language picker (English US,
  Português BR, Español España, Español Latinoamérica = es-419). Only the UI
  is translated, never what you write. The English text is the key
  (`t('…')` / `$t('…')` / `tr('…')` for static lists); catalogs live in
  `frontend/src/locales/`. The choice is per device; the first visit follows
  the browser language. Backend error messages are translated when the exact
  English text is in the catalog, otherwise they show in English.
- Icons: classes can have an uploaded icon; spells can have an uploaded icon or
  one of 17 built-in ones (stored as `builtin:<id>`; the list lives in both
  `frontend/src/builtinIcons.js` and `backend/icons.go` and must match).
- Gear (`backend/gear.go`, `frontend/src/gear.js`): created under Character
  Assets > Gear with a name, weight, slot type, an uploaded or built-in icon
  (16 built-ins) and flat bonuses to any stat. A piece has a slot *type*
  (e.g. Glove); the character sheet has equipment *slots* (Glove 1, Glove 2)
  and any slot of the matching type accepts it. The same piece in two slots
  counts twice. Weight is shown against the carry limit but has no penalty.
  Gear is included in "copy Character Assets".
- Schema version 12. Upgrading keeps all data: 8 -> 9 auto-links old Born in /
  Nation text that matches a location name; 9 -> 10 replaces the unused
  placeholder events tables (carrying over any rows they held); 10 -> 11 adds
  `classes.icon_path`; 11 -> 12 adds the gear tables. The current schema is
  20 (16 -> 17 adds lore articles, 17 -> 18 character tags and dated lore
  rows, 18 -> 19 wiki galleries, 19 -> 20 relation chapters and kingdom
  factions; a migration can run Go code afterwards through `after`); see `migrations` in `backend/db.go`. Character tags
  (`character_tags`) belong to the character, not a version.

## Data on disk

- `DATA_DIR` (`/data` in the container) holds `stories.db` (the list of
  stories) and `stories/<id>/story.db` (one database per story).
- `UPLOADS_DIR` (`/app/uploads`) holds `<id>/...` (each story's pictures and
  icons).
