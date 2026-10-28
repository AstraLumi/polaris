# Stage 5: where the work was paused (2026-10-28)

Delete this file once stage 5 wave 1 is merged. The plan itself is the
stage 5 section at the end of `docs/EDITOR.md`.

## Branches

All local, nothing pushed. `main` is untouched.

| Branch | State |
|---|---|
| `stage5` | `main` + read aloud (merged). Integration branch: merge the others here. |
| `worktree-agent-a9a4b695b1f19dff0` | Read aloud, **done** (`eea6bbd`), already in `stage5`. |
| `worktree-agent-a40abfd6a80825374` | Fonts library, **WIP** commit `43bbdbe`. |
| `worktree-agent-a1f94e0700f2a9c51` | Spellchecker, **WIP** commit `f6b5133`. |
| `worktree-agent-a2cb7333b6af14a47` | Comments, **WIP** commit `0be1a56`. |
| `worktree-agent-ab838363b043402f6` | Tables, **WIP** commit `c1af90a`. |

The worktrees are under `.claude/worktrees/agent-<id>/` (each has its own
`frontend/node_modules`). The WIP commits were made as the agents were
stopped: they are unfinished, untested and may not build. Resume each one in
its worktree, finish it, run the full test list in `CLAUDE.md`, replace the
WIP commit with a real one (amend or a follow-up commit), then merge into
`stage5` with `--no-ff`.

## Ground rules given to every part

- Stay inside the part; changes to shared files (`schema.js`, `pmdoc.go`,
  docx import/export, `WriteToolbar.vue`, `WritePage.vue`, locales) are
  additive so the merges stay easy. Comments and tables both edit the doc
  schema: expect union-style conflicts there and in the locale catalogs,
  `CLAUDE.md`'s test list and `docs/EDITOR.md`.
- **Schema 46 is reserved for comments.** No other part adds a story
  migration (spellcheck stores its words in `app_settings`; tables live in
  the doc JSON; fonts live in the global registry `stories.db`).
- Each part adds a "### <Part> (stage 5)" subsection to the stage 5 section
  of `docs/EDITOR.md` and marks its list item as built.
- Strings in all three locale catalogs; new node tests added to `CLAUDE.md`.
- No real manuscript or user data; test servers on 127.0.0.1.

## Each part when it was stopped

**Fonts library** (item 1; Opus, high effort). Present: `backend/fontparse.go`
(name table + OS/2 fsType parsing) and `fontlib.go` with tests, global routes
wired in `main.go`/`stories.go`, EPUB embedding started in `exportepub.go`,
frontend `views/FontsLibrary.vue` (route + link from Settings),
`FontLicenseModal.vue`, `fontLibrary.js`, `fontRules.js` (warning rules) with
`tests/fontLibrary.test.mjs`, font pickers in `editor/fonts.js`, warnings in
Book/Styles panels and the Export menu. It was fixing a failing test
expectation (characters dropped from a parsed name). Still to check: woff2
handling, "used in" across stories, `@font-face` serving with `nosniff`,
EDITOR.md subsection, locales, full test run. .docx font embedding was
optional.

**Spellchecker** (item 2; Opus, high). Present: gzipped Hunspell
dictionaries en_US, en_GB, es_ES, pt_BR with their licenses under
`frontend/public/dict/` (2.4 MB), `frontend/scripts/dictionaries.mjs` (fetch
script), an own Hunspell reader `editor/hunspell.js` (no nspell dependency so
far), `spell.worker.js`, `spellClient.js`, `spellWords.js` (story names as
words), `backend/spelling.go` (per-story word list in `app_settings`) with
tests. It was about to benchmark load time/memory of the three dictionaries.
Still to do: the benchmark and fixes, the decoration plugin with wavy
underlines, the right-click menu (suggestions / Add to dictionary / Ignore),
the toolbar toggle (turns the browser's spellcheck off), NOTICE.md entries,
node tests (tokenizer, names, language pick, tiny .aff/.dic fixture),
EDITOR.md, locales.

**Comments** (item 3; Opus, high). Present: schema 46 migration in `db.go`
with its undo in `schema16_test.go`, classified in `storyassets.go`,
`backend/comments.go` (routes) with tests, `docxcomments.go` (Word
comments import/export) with tests, `comment` mark in `schema.js` and
`pmdoc.go`, export stripping in `export.go`, `commands.js` started. It was
wiring the .docx import handler (`editorimport.go`) to create comment rows
in the import transaction. Still to do: finish import, round trip in
`TestExportDocxRoundTrip`, the editor side (toolbar button, Ctrl+Alt+M,
highlight, side panel with reply/resolve/delete), the detached-comment
lifecycle (document the choice), node tests, EDITOR.md, locales.

**Tables** (item 5; Opus, high). Present: `prosemirror-tables` added to
`package.json`, `editor/tableSchema.js`, `editor/tables.js`,
`backend/pmtable.go` (validation), changes in `schema.js`, `pmdoc.go`,
`bookLayout.js`, `pageView.js`, `styles.js`, `WriteToolbar.vue`,
`WritePage.vue`, `components/write/TableOptions.vue`,
`tests/editorTables.test.mjs`. It was writing the editor CSS. Still to do:
CSS, Tab between cells without breaking Tab-to-link/list indent, word counts
and chapter index inside cells (Go and JS), .docx import (real `w:tbl`,
gridSpan, vMerge, shading, borders, header rows) and export with round
trip, EPUB/HTML/Markdown/text exports, page view checks with
`writeShots.mjs`, EDITOR.md, locales.

**Read aloud** (item 8; Sonnet, medium): done. Not verified: real voices,
Ctrl+Alt+Space under an OS/IME, phone layout of the floating bar (it
overlaps the page area's bottom edge slightly).

## Not started (wave 2, planned after wave 1 is merged)

Footnotes and endnotes (item 4), boxes and drop caps (6), pictures with
text wrapping (7), offline writing (9). Footnotes, boxes and wrapping all
touch the doc schema and docx code, so run them one after another (or
merge carefully); offline writing should come after the fonts library
(it caches fonts).
