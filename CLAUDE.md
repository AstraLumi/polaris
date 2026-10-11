# Polaris: notes for Claude Code

Self-hosted campaign/character tracker. Go 1.26+ backend (standard library
`net/http`, SQLite via `modernc.org/sqlite`) that embeds the built Vue 3 + Vite
frontend. One SQLite database and one uploads folder per story. See
`README.md` for running it and `docs/DEVELOPMENT.md` for architecture notes.

## Commands

    cd backend  && go vet ./... && go test -count=1 ./...
    cd frontend && npm ci && npm run build
    node frontend/tests/i18n.check.mjs          # must report nothing missing
    node frontend/tests/timelineLayout.test.mjs
    node frontend/tests/markdown.test.mjs
    node frontend/tests/markdownEdit.test.mjs
    node frontend/tests/graphLayout.test.mjs
    node frontend/tests/familyLayout.test.mjs

Run all of these before committing. `backend/static/` is build output (only
`.gitkeep` is tracked); the tests do not need it.

## Rules that are easy to break

- **Database changes are forward-only migrations** in `backend/db.go`
  (`currentSchemaVersion`, `ensureSchema`, `canMigrate`). Never edit an old
  migration. Add a new one, add it to the fresh-schema path, and add a test
  (see `TestMigrationAddsGearTables`). Bump the version. Tests that roll a
  fresh database back to an older schema undo the newest one first with
  `rollBackSchema16` (`backend/schema16_test.go`); a new schema needs its own
  undo step there, or those tests fail with "duplicate column". A new wiki
  article kind gets its delete trigger in its own migration
  (`wikiTriggerSQL`); the schema-13 migration's list is frozen.
- **Every new table must be classified** in `backend/storyassets.go`: either
  `assetTables` (copied with "copy Character Assets") or `storyOnlyTables`.
  `TestEveryTableIsClassified` fails until you do. List image/icon columns in
  `fileCols`.
- **Translations:** the English text is the key. Use `t('…')` in scripts,
  `$t('…')` in templates and `tr('…')` for static lists. Every new string needs
  an entry in all three catalogs in `frontend/src/locales/` (`pt-BR.js`,
  `es-ES.js`; `es-419.js` inherits `es-ES.js` and only overrides words that
  differ). Placeholders such as `{name}` must match. `i18n.check.mjs` fails on
  gaps. Only the interface is translated, never user content.
- **Built-in icons live in two places that must match:**
  `frontend/src/builtinIcons.js` and the allowlists in `backend/icons.go`.
- **Stats:** `stat_modifiers` has source types class, subclass, specialization,
  race, body_type and item (gear). Gear is always flat. `computeStats` is a
  two-phase pipeline (primary, then derived). The derived formulas live in
  `backend/stats_formulas.go`; a user's own go in the git-ignored
  `backend/stats_custom.go` (template: `stats_custom.go.example`). Changing
  `StatInputs` or the stat keys breaks those files, so keep the example and
  the README's "Stat formulas" section in step. Never commit
  `stats_custom.go`.
- **Uploads:** handlers emit `/uploads/<file>`; a rewriter turns that into
  `/uploads/s/<story-id>/<file>`. Uploaded files are served sandboxed with
  `nosniff`. Do not weaken that.
- Dates are `DD-MM-YYYY`, there is no year 0.
- Any file input must read `e.target.files[0]` *before* clearing the input
  (see `dropFile(keepInput)`), or the picker shows "No file chosen".

## Deployment

The server pulls from this repo. Deploying is: commit, push, then run the
update script on the server (`scripts/update.sh` does `git pull` and
`docker compose up -d --build`). Local configuration lives in an untracked
`.env` (see `.env.example`). Never commit `.env`, databases, uploads or
anything with personal data, and never put personal domains or network
addresses in tracked files. The repo is meant to be publishable as is.

## Style

- Match the surrounding code; no new dependencies without a reason.
- Commit messages: a short imperative summary line, then a body if needed.
- Do not push without being asked.
- Versions are Major.Minor.Bugfix (semver), kept in `frontend/package.json`
  and tagged `vX.Y.Z`. New features bump Minor, fixes bump Bugfix.
