# The sky from a hex: roadmap and guidelines

The goal: click any hex of the map, pick a date (and time), and see the
night (or day) sky from there, true enough that art made from it stays
consistent and stars can be charted and named: a star named in a scene is
really in that part of the sky, at that time of year, from that place.

This file is the plan every part of the solar system, the map's climate
and the sky is built towards. Read it before working on any of them.

## Where we are

Already true to scale, already reusable:

- `orbits.js` and `solarSimulation.js` work in real units (km, kg, s;
  double precision). Kepler orbits with eccentricity and inclination,
  live n-body gravity in a worker, positions at any day. The solar view's
  squeezed scale (`solarScale.js`) is only a picture of that state.
- `stars.js`: luminosity, temperature colour, habitable zone.
- `three/shadows.js` / `eclipses.js`: the disc-overlap maths of eclipses
  (reused for eclipses seen from the ground).
- The planet grid (`planetGrid.js`): every hex has a centre on the unit
  sphere, i.e. a latitude and longitude. 2D maps have latitudes per row
  (seasons and latitudes, schema 26).
- `seasons.js`: seasons from the tilt; `climate.js`:
  `mainPlanetCloudCover`, the hook for real clouds.
- The sky picture format (equirectangular, `cmd/skygen`, `PlanetGlobe.vue`'s
  sky shader).
- `frontend/tests/visual/shots.mjs`: headless screenshots, so every visual
  step can be checked.

Missing for the goal:

1. **Time.** No link between a calendar date and the simulation's day 0
   (an epoch), and no time of day.
2. **The planet's orientation.** Tilt is a number; there is no direction
   the axis points in space (which decides when the solstices fall), no
   rotation angle at day 0 (where noon is), no sidereal vs solar day.
3. **Stars as data.** The background is a painted picture. Charting and
   naming need a catalogue: every star with a fixed direction, a
   brightness (magnitude), a colour, an id, maybe a name.
4. **Atmospheres** as physical parameters (pressure, composition).
5. **Lighting other than one star** (the enclosed, glowing shell).
6. **Climate**: temperature, wind, rain, clouds per hex and season.

## Guidelines (for every part)

- **One truth, many pictures.** The physical state is
  `solarSimulation.js` in real units. The solar view, the 2D editor and
  the sky view are projections of it. Nothing physical is computed in
  scene units; `solarScale.js` stays drawing-only.
- **The sky is directions, not distances.** The sky view never puts
  km-scale coordinates on the GPU (float32 can't hold them). Positions are
  computed in double precision in JS, then turned into directions and
  angular sizes as seen from the observer; only those go to the shader.
- **Deterministic and versioned.** Anything generated (star catalogue,
  climate, clouds) comes from stored seeds and settings through versioned
  generators, so a story's sky never changes unless the author changes
  it. Never `Math.random()` in anything that feeds the sky. When a
  generator changes, old stories keep the old version (`sky_version`).
- **The sky uses Kepler orbits.** Any date must give the same sky,
  quickly. Live gravity is for watching what-ifs; it drifts and costs
  time over long spans. If gravity should ever shape the sky, it must
  be integrated once and stored.
- **Pure modules with node tests** for all physics and generation
  (`skyFrames`, `starCatalog`, `atmosphere`, `climate`, `lighting`), no
  three.js in them. Drawing code only draws.
- **What defines the world is stored, what follows is derived.** The
  epoch, the axis, atmospheres, the star seed, named stars, the enclosure
  go in the database through migrations (CLAUDE.md rules). Derived data
  (climate, cloud maps, rendered skies) is recomputed or cached, never
  the source of truth.
- **Heavy work in workers**, interactive previews fast, final renders
  progressive (they may take seconds).
- **Every visual step is checked with `shots.mjs`**, extended with a sky
  mode (`--sky <hex>,<date>,<time>`), and servers bound to 127.0.0.1.

## Answers to the open design questions

**True scale.** Already the case for the physics; the plan keeps it.
The squeezed solar view stays as an editor. The sky view is a separate
renderer working from true positions (as directions), so a moon shows its
true angular size and a planet is a point.

**The sky is drawn by the author; stars are found in it.** The sky
picture (the equirectangular texture) stays what the author controls:
drawn by hand, or generated and then edited. The app reads the stars out
of it: small bright spots standing out from their surroundings are
detected, and each gives a direction (its centre), a brightness (its light
summed over the spot) and a colour. That is the catalogue. What is left
once the stars are taken out is the diffuse glow (the galaxy's band,
nebulae), used as emissive light. A brightness scale set by the author
maps pixels to magnitudes (an 8-bit picture has little range: "the
brightest star I drew is as bright as Sirius"). An author can also give
two pictures, stars only and glow only, to need no guessing.

Why not the texture alone as emissive light: it would look right in one
picture, but stars couldn't be named or charted, dimmed one by one by the
atmosphere near the horizon, kept sharp at any resolution, or hidden by
twilight (a pixel can't fade below black). So stars are drawn as points
from the catalogue, the glow as a texture, and rendering is HDR, then
exposed and tone mapped like a camera or an eye.

**Atmosphere from composition.** Reasonable. Pressure, gravity
(known), temperature and the gas mix give the Rayleigh scattering
(the sky's colour), plus haze/dust (Mie: whiteness, sunsets) and absorbers
(ozone; methane and others tint it). Presets (Earth-like, thin Mars-like,
thick hazy, methane) with free values behind them. Precomputed scattering
tables (Bruneton/Hillaire style) make it fast.

**Ray tracing.** Yes, for the sky view: per pixel, a ray from the
observer is marched through the atmosphere, tested against the moons and
planets as exact spheres at true scale (their phases and eclipses fall
out of it), against the cloud layer, and ends on the star catalogue or
the shell. On WebGL2, a progressive full-screen shader that accumulates
samples over a few seconds; a preview with fewer samples. Not path tracing
the whole scene, which isn't needed.

**The Dyson world.** It needs a general *lighting model* instead of
"the star lights things". Two models:

- `star` (now): light from the star's disc, shadows, seasons from the
  tilt.
- `shell`: the system is enclosed in a shell whose inside glows. Light
  comes from every direction with a 24-hour brightness cycle that is the
  same everywhere at once. There are no time zones and no phases (moons
  are lit all round). Seasons come only from whatever the shell does.
  The night sky is the shell itself (its pattern, its lights), not stars.

Every consumer asks the lighting model, not the star: the solar view's
shading, the climate's sunlight (uniform over latitude for the shell),
and the sky view's background and brightness. A system-level setting,
`enclosure`: `none`, `star` (today's Dyson flag) or `shell` with its
cycle and look. Details to settle with the author when we get there: how
the shell looks from inside, whether the cycle is a uniform dimming or a
band moving across, and whether anything shows through.

**Star charting.** The catalogue gives every star an id and a fixed
direction in the system's frame (they are too far for parallax). Names,
constellations (lines between ids) and notes are stored per story, tied
to a star's direction: when the picture is redrawn and scanned again, each
name moves to the nearest star found (within a small angle), so editing
the sky keeps the names; one that loses its star is flagged. The
sky view labels them, and printable charts (per hemisphere and season)
come from the same data.

## Stages

Each stage is small enough to push on its own and leaves something
usable.

1. **Time and frames** (`skyFrames.js`, schema): a calendar date and time
   to simulation days (an epoch setting, e.g. the date of day 0, or the
   date of a solstice); the main planet's axis direction and rotation
   angle at day 0; sidereal day from the solar day; a hex (or a 2D map's
   row latitude) to latitude/longitude to a local horizon frame (altitude
   and azimuth). `seasons.js` follows the same orbit and axis so the
   timeline's seasons and the sky agree. Tests: the sun is highest at
   local noon, equinoxes and solstices fall where the calendar says.
2. **The star catalogue** (`starCatalog.js`, schema): stars detected in
   the story's sky picture (an uploaded drawing, or the generated one:
   skygen keeps drawing it from a seed, and can write the catalogue
   directly), with the author's brightness scale; the glow left over as
   its own texture; named stars and constellations stored per story,
   re-matched on every new picture. Detection is deterministic and
   versioned like any generator. Tests: stars drawn at known places and
   brightnesses are found there, in the right order of brightness.
3. **Sky view, airless**: a "Sky" button on a hex with a date and time
   picker; renders stars, the sun, moons (true size, phase, eclipses),
   planets, horizon, compass and an alt/az grid, with star labels. Uses
   Kepler positions. `shots.mjs --sky`.
4. **Atmospheres**: fields per planet (pressure, composition preset and
   values, haze), scattering tables, blue skies and twilight, star
   extinction and the sky's brightness hiding faint stars, moonlight.
5. **Lighting models**: `enclosure` (none, star, shell), the shell's
   cycle and look, used by the solar view, the sky view and climate.
6. **Climate** (`climate.js` grows, in a worker): sunlight per latitude
   and season from the lighting model; temperature from it, altitude
   (mountain layer), oceans (depth layer) and land cover; prevailing
   winds from circulation cells (their number depends on the day's
   length); moisture carried by the winds, rain on windward slopes, rain
   shadows; clouds per hex and season. Feeds `mainPlanetCloudCover`, the
   map, and the sky view's cloud layer. Results cached per map version.
7. **Final renders**: progressive ray-marched sky, a high-resolution
   galactic band, star glow (a point spread function and bloom), export
   at a chosen size for art.
8. **Charting extras**: printable star charts, "what is up tonight" lists,
   rise and set times, a star's yearly visibility from a hex.

Stages 1–3 make the goal work in its simplest form (an airless sky from
a hex on a date). 4 and 6 make it look like a real place; 5 is the
author's own world; 7 and 8 are polish.
