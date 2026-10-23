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
    node frontend/tests/seasons.test.mjs
    node frontend/tests/planet.test.mjs
    node frontend/tests/planetGrid.test.mjs
    node frontend/tests/bucketFill.test.mjs
    node frontend/tests/orbits.test.mjs
    node frontend/tests/solarSimulation.test.mjs
    node frontend/tests/eclipses.test.mjs
    node frontend/tests/climate.test.mjs
    node frontend/tests/oceanNames.test.mjs
    node frontend/tests/climateModel.test.mjs
    node frontend/tests/flowLines.test.mjs
    node frontend/tests/lighting.test.mjs
    node frontend/tests/skyFrames.test.mjs
    node frontend/tests/starCatalog.test.mjs
    node frontend/tests/bodyOutlines.test.mjs

Run all of these before committing.

To see the Solar system's 3D view without a screen (headless Edge, a
throwaway story, never real data), take screenshots and look at them:

    node frontend/tests/visual/shots.mjs --views angle,side,Giant [--days 60 --gravity --star black_hole]

Options are listed at the top of the script; the pictures land in
`frontend/tests/visual/out/` (ignored by git). The camera views come from
`window.__solarScene` / `window.__solarPage`, defined only on the dev
server. The map's continents and climate layers have their own script,
on a rough Earth (`frontend/tests/earthLike.mjs`):

    node frontend/tests/visual/climateShots.mjs --layers continents,wind,temperature [--month 7 --flat]
 `backend/static/` is build output (only
`.gitkeep` is tracked); the tests do not need it.

## Rules that are easy to break

- **Database changes are forward-only migrations** in `backend/db.go`
  (`currentSchemaVersion`, `ensureSchema`, `canMigrate`). Never edit an old
  migration. Add a new one, add it to the fresh-schema path, and add a test
  (see `TestMigrationAddsGearTables`). Bump the version. Tests that roll a
  fresh database back to an older schema undo the newest one first with
  `rollBackSchema16` (`backend/schema16_test.go`); a new schema needs its own
  undo step there, or those tests fail with "duplicate column". Before
  2.0.1 foreign keys were enforced on one pooled connection only, so real
  databases hold rows whose parent is gone: a migration that copies rows
  into a table with a foreign key must skip those (`TestMigrationsSkipLeftoverRows`). A new wiki
  article kind gets its delete trigger in its own migration
  (`wikiTriggerSQL`); the schema-13 migration's list is frozen. A kind
  whose table is created after the wiki's in the fresh schema sets
  `OwnTrigger` and makes its trigger in that table's migration.
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
  a year alone is emitted as just the year. Its calendar button opens a
  month of the story's calendar with the seasons tinted in their colours.
- **The map is 2D only or a planet** (`map_mode`, `backend/planetmap.go`).
  The 2D tables (`location_hexes`, `locations.q/r`, `creature_hexes`) and
  the planet's (`planet_cells`, `locations.cell`, `creature_cells`) are kept
  side by side; anything that reads positions must ask `mapMode`/`spaceOf`
  (`backend/positions.go`) which one counts. The planet's grid and the map
  projections exist twice, in Go (`planetgrid.go`, `projection.go`) and JS
  (`planetGrid.js`, `projection.js`): change both, and keep the shared
  fixtures in their tests equal. The planet's layers (tectonic plates,
  continents, ocean depths: `map_features`/`feature_cells`,
  `backend/planetlayers.go`, `planetLayers.js`) are planet-only, one
  feature per layer per cell; each hex can also carry a biome
  (`cell_biomes`: what covers it) and, apart from it, a terrain
  (`cell_relief`: hills, plateau, mountains, with an optional ground
  height), so a plateau can be forested. Biome and terrain keys exist
  twice, `frontend/src/biomes.js` (`BIOME_GROUPS`, `LANDFORMS`) and
  `biomeKeys`/`reliefKeys` in `planetlayers.go`: change both. All of it must be resampled with everything else when the hex
  size changes (`resampleLayers`).
- **The climate** (winds, ocean currents, temperature, rain, Köppen types)
  is derived, never stored: `frontend/src/climateModel.js` works it out
  from land, biomes, heights, tilt, day length and the star
  (`planetClimate.js` gathers those, runs it in `climate.worker.js` and
  reads it on the hexes). Keep it a pure module with no drawing; its
  constants are calibrated against the rough Earth in
  `frontend/tests/earthLike.mjs` (`climateModel.test.mjs` checks the
  patterns an atlas shows). High ground steers the winds
  (`steerByTerrain`: along ranges, through passes, calm in the lee), read
  from each climate cell's ridges, not its mean height, so thin ranges
  count (a walled test planet checks it). Feedbacks between rounds: the
  air's water keeps heat in (dry air lets it go), heavy rain lowers the
  pressure, lee slopes warm (foehn), freezing high slopes pour katabatic
  winds, volcanic biomes give ground heat (`heat` in `biomes.js`). Arrows: paths in `flowLines.js`, meshes in
  `three/flowArrows.js`.
- **Light and heat are separate** (`frontend/src/lighting.js`): ask it,
  not the star, for what lights a body (directional from the star, even
  from an enclosing shell) and what warms it (the star's heat gets out of
  a Dyson sphere; light doesn't). The climate uses heat, the drawing
  light. The system can sit inside a shell that is its sky (a switch,
  `enclosed` from `GET /api/solar`, made into an enclosure by
  `enclosureFor`): even light on the main planet's day and night, only
  the painted stars' light at night; moons then have no phases
  (`moonlightOf`). A moon can shine on its own (`glows`, schema 41):
  bright as a full moon whatever lights it (`MOON_GLOW`).
- **The sky view** (`SkyView.vue`, "Sky from this hex" on the planet map):
  time and frames in `skyFrames.js` (day 0 = 01-01-0001, axis from the
  solstice rule shared with the climate, sidereal spin, the sky picture
  fixed to the stars at day 0), stars found in the sky picture by
  `starCatalog.js`, drawing in `three/skyShaders.js`. Positions are Kepler
  orbits in km, double precision; only directions and angular sizes reach
  the GPU. Screenshots: `climateShots.mjs --sky lon,lat --at
  DD-MM-YYYY,hour --look az,alt,fov`. three.js is only loaded with the planet
  (`MapPage.vue`, `PlanetTerritory.vue` load it lazily); keep it out of the
  main bundle.
- **The solar system** (`backend/solar.go`, the Solar system tab) keeps
  the bodies only; their physics lives in `frontend/src/orbits.js` (mass
  from size and one density per type, gravity, orbits behind a swappable
  orbit model) and the picture's squeezed scale in `solarScale.js`. Keep
  physics out of the drawing code. The main planet's size is the map's
  (`planet_km`), its tilt the map's, its day the calendar's hours per day,
  and its orbit radius follows from the calendar's year and the star's mass
  (`setMainOrbit` in `solar.go`, which repeats the few lines of physics it
  needs); none of these is stored on the body. The globe mesh and the sky are shared
  three.js modules (`src/three/`) used by the map's globe and the solar
  system; only lazily loaded pages may import them. Planets and moons other
  than the main one have painting-only maps (`body_cells`,
  `backend/bodymaps.go`, `BodyMap.vue`), opened from the Map page's planet
  switch; the main planet's map is the story's map. Planets' clouds come
  from `frontend/src/climate.js`; the main planet's through
  `mainPlanetCloudCover` (one cover per hex), the hook meant to read the
  wind, mountain, humidity and temperature maps later.
- **The sky format** (an equirectangular 2:1 picture, top above the north
  pole, middle column above longitude 0, east to the left) is defined twice:
  by `backend/cmd/skygen`, which draws the default sky
  (`frontend/public/sky/default-sky.jpg`, also the downloadable template:
  `cd backend && go run ./cmd/skygen ../frontend/public/sky/default-sky.jpg`),
  and by the sky shader in `PlanetGlobe.vue`. Change both together.
- **The height chart** (`HeightChart.vue`, `/characters/heights`) draws
  each picked version at its height with a plain outline from
  `frontend/src/bodyOutlines.js` (sex, age, build; pure, tested). A body
  type stores its outline (`outline_sex/age/build` on `body_types`,
  schema 40, `backend/bodyoutlines.go`); an empty part comes from the
  character (gender, age). The keys exist in both files: change both.
  Screenshots: `node frontend/tests/visual/heightShots.mjs [--ft]` (also
  the date picker's calendar).
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
