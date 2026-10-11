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
    node frontend/tests/dateDigits.test.mjs
    node frontend/tests/storyTime.test.mjs
    node frontend/tests/formulaBlocks.test.mjs

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
- **Stats:** each story defines its own stats and formulas (`stat_defs`,
  `stat_settings`; `backend/statsystem.go`, edited in Settings → Stats).
  Nothing in the code may assume a stat key exists: the sheet, bonus forms,
  compare page and card all read the stat system (`frontend/src/stats.js`).
  The five categories are fixed and keep their behaviour (primary = points,
  base = per-level bonuses from assets, gear always flat). Formulas are text
  parsed by `backend/statformula.go`; `frontend/src/formulaBlocks.js` reads
  the same syntax as blocks, so change both together. `defaultStatSystem()`
  must keep giving the pre-2.0 numbers (`TestDefaultStatSystemMatchesTheOldFormulas`).
  Typed numbers (points and [base]) live in `character_special_bases` by key.
- **Uploads:** handlers emit `/uploads/<file>`; a rewriter turns that into
  `/uploads/s/<story-id>/<file>`. Uploaded files are served sandboxed with
  `nosniff`. Do not weaken that.
- Dates are `DD-MM-YYYY`, there is no year 0. They are typed in one box
  (`components/DateField.vue`): digits are typed and shown left to right
  but read from the right, the last four being the year (`dateDigits.js`);
  a year alone is emitted as just the year.
- Birth date, blood type and Born in belong to the character (`characters`
  columns, schema 23), not to a version; the version form edits them for
  all versions.
- Any file input must read `e.target.files[0]` *before* clearing the input
  (see `dropFile(keepInput)`), or the picker shows "No file chosen".

## Deployment

The server pulls from this repo. Pushing a `v*` tag makes
`.github/workflows/release.yml` publish `ghcr.io/astralumi/polaris` (X.Y.Z,
X.Y, latest; amd64 and arm64). Deploying is: release, then run the update
script on the server (`scripts/update.sh` does `git pull`, `docker compose
pull` and `up -d`; `--build` builds from the code instead, needed for changes
pushed without a release). Local configuration lives in an untracked
`.env` (see `.env.example`). Never commit `.env`, databases, uploads or
anything with personal data, and never put personal domains or network
addresses in tracked files. The repo is meant to be publishable as is.

## Style

- Match the surrounding code; no new dependencies without a reason.
- Commit messages: a short imperative summary line, then a body if needed.
- Do not push without being asked.
- Versions are Major.Minor.Bugfix (semver), kept in `frontend/package.json`
  (and the two top `version` fields of `package-lock.json`). New features
  bump Minor, fixes bump Bugfix.

## Pushing and releases

- **"push"** on its own means commit and push to `main`, nothing more: no
  version bump, no tag, no release.
- **"push as X.Y.Z"** (any push that names a version) is a release:
  1. set the version to X.Y.Z;
  2. add a `## X.Y.Z (YYYY-MM-DD)` section at the top of `CHANGELOG.md`
     explaining the changes since the last release, in plain words for the
     user, under `### Added` / `### Changed` / `### Fixed`. When the
     schema changed, start it with the `**Upgrade:**` line used in earlier
     entries (stories upgrade on first open, it can't be undone, export
     first);
  3. commit, tag `vX.Y.Z`, push `main` and the tag;
  4. `gh release create vX.Y.Z --title "Polaris X.Y.Z" --notes-file <file>`
     with that same section as the notes (it becomes the latest release).
- If changes were pushed without a release since the last one, the next
  release's notes cover them too.
