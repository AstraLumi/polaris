# README screenshots

Retakes the screenshots in `docs/screenshots/` from a demo story of invented
data ("The Ashen Crown"). Needs Node 24+ and Edge or Chrome; no packages.

- `seed-demo.js` builds the demo story through the API (it runs inside the
  page, so it can draw the pictures on a canvas): a map with four kingdoms,
  ten characters with versions, relations and an adoption, factions, events,
  chapters, lore, a bestiary and wiki text.
- `shoot.mjs` starts a headless browser, runs the seed and saves
  `wiki.png`, `graph.png`, `family.png`, `timeline.png`, `map.png`,
  `character.png`, `bestiary.png` and `stats.png` (1440 wide). Edit it to
  add or change shots.

Run it against a fresh server with throwaway folders, never your real data
(it adds a story wherever it points):

```bash
cd frontend && npm ci && npm run build && cp -r dist/. ../backend/static/ && cd ..
cd backend && go build -o /tmp/polaris-demo . && cd ..
mkdir -p /tmp/demo/data /tmp/demo/up
DATA_DIR=/tmp/demo/data UPLOADS_DIR=/tmp/demo/up PORT=8096 /tmp/polaris-demo &
node scripts/screenshots/shoot.mjs http://localhost:8096 docs/screenshots
```

Then stop the server and empty `backend/static/` again (only `.gitkeep` is
tracked). If the browser isn't Edge at its usual Windows path, set `BROWSER`
to the browser's executable.
