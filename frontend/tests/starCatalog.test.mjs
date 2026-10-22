// Finding the stars in the sky picture (src/starCatalog.js). No dependencies:
// node frontend/tests/starCatalog.test.mjs
import assert from 'node:assert/strict'
import { findStars, skyDirection, skyPixel, CATALOGUE_VERSION } from '../src/starCatalog.js'

// A small deterministic random generator.
function rng(seed) {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
const degrees = (a, b) => (Math.acos(Math.min(1, dot(a, b))) * 180) / Math.PI

// A dark sky: a noisy background with a smooth bright band across it like a
// galaxy (and, if asked, moons' discs: { x, y, r, crescent }), as RGBA bytes.
function backdrop(W, H, { seed = 1, discs = [] } = {}) {
  const data = new Uint8ClampedArray(W * H * 4)
  const rand = rng(seed)
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) {
      const across = (y - H / 2 - 0.3 * H * Math.sin((2 * Math.PI * x) / W)) / (0.09 * H)
      const band = 45 * Math.exp(-across * across)
      const j = (y * W + x) * 4
      const n = (rand() + rand() + rand() - 1.5) * 2.4 // about 1.2 grey levels of noise
      data[j] = 6 + 0.6 * band + n
      data[j + 1] = 8 + 0.8 * band + n
      data[j + 2] = 14 + band + n
      data[j + 3] = 255
    }
  }
  for (const d of discs) {
    for (let y = Math.floor(d.y - d.r - 2); y <= d.y + d.r + 2; y++) {
      for (let x = Math.floor(d.x - d.r - 2); x <= d.x + d.r + 2; x++) {
        const dist = Math.hypot(x - d.x, y - d.y)
        if (dist > d.r) continue
        const j = (y * W + x) * 4
        // The lit part is a full disc, or a crescent (the disc less a shifted
        // one), the rest a dim earthshine; the surface is mottled.
        const lit = !d.crescent || Math.hypot(x - d.x - 0.5 * d.r, y - d.y) > d.r * 1.05
        const v = lit ? 150 + 25 * Math.sin(x * 0.7) * Math.cos(y * 0.9) : 18
        data[j] = v
        data[j + 1] = v * 0.95
        data[j + 2] = v * 0.9
      }
    }
  }
  return { width: W, height: H, data }
}

// Paints a round star on the sky at pixel (x, y) (fractions allowed): a
// Gaussian of angular size sigma (radians) and the given peak, as it falls on
// the picture, stretched sideways near the poles.
function paintStar(img, s) {
  const { width: W, height: H, data } = img
  const c = skyDirection(s.x, s.y, W, H)
  const lat = Math.asin(c[2])
  const ry = Math.ceil(5 * (s.sigma / Math.PI) * H) + 1
  const rx = Math.min(Math.floor(W / 2), Math.ceil(ry / Math.max(Math.cos(lat), 1e-4)) + 1)
  const cx = Math.round(s.x)
  const cy = Math.round(s.y)
  for (let y = Math.max(0, cy - ry); y <= Math.min(H - 1, cy + ry); y++) {
    for (let dx = -rx; dx <= rx; dx++) {
      const x = (((cx + dx) % W) + W) % W
      const a = Math.acos(Math.min(1, dot(skyDirection(x, y, W, H), c)))
      const k = s.peak * Math.exp((-a * a) / (2 * s.sigma * s.sigma))
      if (k < 0.05) continue
      const j = (y * W + x) * 4
      for (let ch = 0; ch < 3; ch++) data[j + ch] += k * s.color[ch]
    }
  }
}

// The light a drawn star adds up to (the Gaussian's integral on the sky).
const lightOf = (s) => s.peak * s.sigma * s.sigma

function makeStars(W, H, n, seed, { sigmaRange, peakRange }) {
  const rand = rng(seed)
  const colors = [[1, 1, 1], [0.8, 0.9, 1], [1, 0.85, 0.65]]
  const out = []
  for (let k = 0; k < n; k++) {
    // Even over the sphere, away from the very poles (those are added by hand).
    const lat = Math.asin(rand() * 1.8 - 0.9)
    const x = rand() * W
    const y = (0.5 - lat / Math.PI) * H - 0.5
    const pxRad = Math.PI / H // one pixel, in radians
    out.push({
      x,
      y,
      sigma: (sigmaRange[0] + rand() * (sigmaRange[1] - sigmaRange[0])) * pxRad,
      peak: peakRange[0] + rand() * (peakRange[1] - peakRange[0]),
      color: colors[k % colors.length],
    })
  }
  return out
}

// ---- The picture's geometry --------------------------------------------------

{
  const W = 2048
  const H = 1024
  // The middle column at the equator looks at +x.
  const mid = skyDirection(W / 2 - 0.5, H / 2 - 0.5, W, H)
  assert.ok(degrees(mid, [1, 0, 0]) < 1e-9)
  // A column left of the middle is east: positive longitude, +y.
  assert.ok(skyDirection(W / 2 - 100.5, H / 2 - 0.5, W, H)[1] > 0)
  assert.ok(skyDirection(W / 2 + 99.5, H / 2 - 0.5, W, H)[1] < 0)
  // The top row is the north, the bottom the south.
  assert.ok(skyDirection(5, 0, W, H)[2] > 0.999)
  assert.ok(skyDirection(5, H - 1, W, H)[2] < -0.999)
  // The left edge is a quarter turn short of a full turn from the right.
  const left = skyDirection(0, H / 2 - 0.5, W, H)
  const right = skyDirection(W - 1, H / 2 - 0.5, W, H)
  assert.ok(degrees(left, right) < 360 / W * 1.0001)
  // And back again.
  const rand = rng(5)
  for (let k = 0; k < 200; k++) {
    const x = rand() * W
    const y = rand() * (H - 1)
    const [bx, by] = skyPixel(skyDirection(x, y, W, H), W, H)
    assert.ok(Math.abs(bx - x) < 1e-6 && Math.abs(by - y) < 1e-6, `round trip ${x},${y} -> ${bx},${by}`)
  }
  // A direction not of unit length still lands in the same place.
  const [ux, uy] = skyPixel([0, 3, 0], W, H)
  assert.ok(Math.abs(ux - (W / 4 - 0.5)) < 1e-9 && Math.abs(uy - (H / 2 - 0.5)) < 1e-9)
}

// ---- Stars drawn at known places and brightnesses are found there -----------------

const W = 2048
const H = 1024
const pxRad = Math.PI / H
const drawn = [
  ...makeStars(W, H, 34, 11, { sigmaRange: [1.0, 1.5], peakRange: [45, 220] }),
  // Near the poles, where the picture stretches stars sideways.
  { x: 700.3, y: (0.5 - 80 / 180) * H - 0.5, sigma: 1.2 * pxRad, peak: 160, color: [1, 1, 1] },
  { x: 1500.7, y: (0.5 - 88 / 180) * H - 0.5, sigma: 1.2 * pxRad, peak: 110, color: [1, 0.9, 0.8] },
  { x: 300.2, y: (0.5 + 85 / 180) * H - 0.5, sigma: 1.0 * pxRad, peak: 200, color: [0.8, 0.9, 1] },
  // On the seam, where the left and right edges meet.
  { x: W - 0.6, y: 400.4, sigma: 1.3 * pxRad, peak: 140, color: [1, 1, 1] },
  { x: 0.3, y: 650.5, sigma: 1.1 * pxRad, peak: 180, color: [1, 1, 1] },
  // A bright one, to be the brightest.
  { x: 1000.5, y: 500.5, sigma: 1.5 * pxRad, peak: 250, color: [1, 1, 1] },
]

{
  const img = backdrop(W, H)
  for (const s of drawn) paintStar(img, s)
  const { stars } = findStars(img)
  assert.ok(stars.length >= drawn.length && stars.length <= drawn.length + 1, `found ${stars.length} of ${drawn.length}`)

  // Each drawn star has a found star within a fraction of a degree, no two sharing one.
  const taken = new Set()
  const match = drawn.map((s) => {
    const d = skyDirection(s.x, s.y, W, H)
    let best = -1
    let bestAngle = 1e9
    stars.forEach((f, i) => {
      const a = degrees(f.dir, d)
      if (a < bestAngle) {
        bestAngle = a
        best = i
      }
    })
    assert.ok(bestAngle < 0.25, `star at ${s.x},${s.y} found ${bestAngle} degrees off`)
    assert.ok(!taken.has(best), 'one found star for two drawn')
    taken.add(best)
    return best
  })

  // Brightest first.
  for (let i = 1; i < stars.length; i++) assert.ok(stars[i].mag >= stars[i - 1].mag)
  assert.equal(stars[0].mag, -1.46)
  assert.equal(match[drawn.length - 1], 0, 'the brightest drawn is the first found')

  // Pairs that differ by more than half a magnitude come out in that order, and
  // the differences match 2.5 log10 of the light ratios.
  for (let i = 0; i < drawn.length; i++) {
    for (let j = 0; j < drawn.length; j++) {
      const expected = 2.5 * Math.log10(lightOf(drawn[i]) / lightOf(drawn[j]))
      const got = stars[match[j]].mag - stars[match[i]].mag
      if (expected > 0.5) assert.ok(got > 0, `order of ${i} and ${j}`)
      assert.ok(Math.abs(got - expected) < 0.3, `magnitude difference ${got} against ${expected} (${i}, ${j})`)
    }
  }

  // The colours lean the way they were drawn (a blue one is bluer than red).
  for (let i = 0; i < drawn.length; i++) {
    assert.match(stars[match[i]].color, /^#[0-9a-f]{6}$/)
  }
  const blue = drawn.findIndex((s) => s.color[0] === 0.8)
  const c = stars[match[blue]].color
  assert.ok(parseInt(c.slice(5, 7), 16) > parseInt(c.slice(1, 3), 16), `blue star colour ${c}`)
  const orange = drawn.findIndex((s) => s.color[2] === 0.65)
  const o = stars[match[orange]].color
  assert.ok(parseInt(o.slice(1, 3), 16) > parseInt(o.slice(5, 7), 16), `orange star colour ${o}`)

  // Where the stars were, the glow has none left, and elsewhere it is the picture.
  const clean = backdrop(W, H)
  const { glow } = findStars(img)
  assert.equal(glow.width, W / 2)
  assert.equal(glow.height, H / 2)
  assert.equal(glow.data.length, (W / 2) * (H / 2) * 4)
  const half = (data, x, y, k) => {
    const a = (2 * y * W + 2 * x) * 4 + k
    return (data[a] + data[a + 4] + data[a + W * 4] + data[a + W * 4 + 4]) / 4
  }
  let total = 0
  for (let y = 0; y < H / 2; y++) {
    for (let x = 0; x < W / 2; x++) {
      total += Math.abs(glow.data[(y * (W / 2) + x) * 4 + 2] - half(clean.data, x, y, 2))
    }
  }
  // (Without the stars the average is a fraction of a level off, from the noise.)
  assert.ok(total / ((W / 2) * (H / 2)) < 0.5, 'the glow is the picture where there are no stars')
  for (const s of drawn) {
    const gx = Math.round(s.x / 2)
    const gy = Math.round(s.y / 2)
    const rx = Math.abs(s.y - H / 2) > H * 0.45 ? 40 : 5
    for (let y = Math.max(0, gy - 5); y <= Math.min(H / 2 - 1, gy + 5); y++) {
      for (let dx = -rx; dx <= rx; dx++) {
        const x = (((gx + dx) % (W / 2)) + W / 2) % (W / 2)
        const left = glow.data[(y * (W / 2) + x) * 4 + 2]
        const want = half(clean.data, x, y, 2)
        assert.ok(Math.abs(left - want) < 14, `a star's light is left in the glow at ${x},${y}: ${left} against ${want}`)
      }
    }
  }
  // The band is still there: the glow along the band is far above the sky off it.
  const gl = (x, y) => glow.data[(y * (W / 2) + x) * 4 + 2]
  assert.ok(gl(W / 4, Math.round(H / 4)) > gl(W / 4, 10) + 30, 'the band stays in the glow')

  // The same picture gives the same catalogue.
  const again = findStars(img)
  assert.deepEqual(again.stars, stars)
  assert.deepEqual(again.glow.data, glow.data)

  // Options: only the brightest few; a different scale.
  assert.equal(findStars(img, { maxStars: 5 }).stars.length, 5)
  assert.ok(Math.abs(findStars(img, { brightest: 0 }).stars[0].mag) < 1e-9)
  const faint = findStars(img, { faintest: 0.5 }).stars
  assert.ok(faint.length > 3 && faint.length < stars.length)
  assert.ok(faint.every((s) => s.mag <= 0.5))
}

// ---- Nothing in the band and the noise; a moon's disc is no star, and not in the glow ----

{
  // A full disc off the band, and a crescent (its dark side dim) on the band.
  const moons = [
    { x: W * 0.3, y: H * 0.3, r: H * 0.03 },
    { x: W * 0.6, y: H / 2 + 0.3 * H * Math.sin(2 * Math.PI * 0.6), r: H * 0.04, crescent: true },
  ]
  const img = backdrop(W, H, { discs: moons })
  const clean = backdrop(W, H)
  const { stars, discs, glow } = findStars(img)
  assert.equal(stars.length, 0, `${stars.length} stars found in an empty sky`)
  assert.equal(discs.length, 2)
  for (const m of moons) {
    const d = discs.find((c) => Math.hypot(c.x - m.x, c.y - m.y) < 3)
    assert.ok(d, `the disc at ${m.x},${m.y} is found`)
    assert.ok(Math.abs(d.radius - (m.r * Math.PI) / H) < (3 * Math.PI) / H, 'its radius ' + (d.radius * H / Math.PI) + ' against ' + m.r)
    assert.ok(degrees(d.dir, skyDirection(d.x, d.y, W, H)) < 1e-9)
    // Its pixels in the glow are what the sky was without it (to within what
    // a straight line across the band's curve can be).
    for (let y = Math.floor(m.y / 2 - m.r / 2); y <= m.y / 2 + m.r / 2; y++) {
      for (let x = Math.floor(m.x / 2 - m.r / 2); x <= m.x / 2 + m.r / 2; x++) {
        for (let k = 0; k < 3; k++) {
          const want = (clean.data[(2 * y * W + 2 * x) * 4 + k] + clean.data[(2 * y * W + 2 * x + 1) * 4 + k]) / 2
          assert.ok(Math.abs(glow.data[(y * (W / 2) + x) * 4 + k] - want) < 25, `a moon is left in the glow at ${x},${y}: ${glow.data[(y * (W / 2) + x) * 4 + k]} against ${want}`)
        }
      }
    }
  }
  // The band is still in the glow, even by the moons.
  const gl = (x, y) => glow.data[(y * (W / 2) + x) * 4 + 2]
  assert.ok(gl(W / 4, Math.round(H / 4)) > gl(W / 4, 10) + 30, 'the band stays in the glow')
  // And a star next to the disc still turns up.
  const s = { x: W * 0.3 + H * 0.03 + 12, y: H * 0.3, sigma: 1.2 * pxRad, peak: 150, color: [1, 1, 1] }
  paintStar(img, s)
  const found = findStars(img)
  assert.equal(found.stars.length, 1)
  assert.equal(found.discs.length, 2)
  assert.ok(degrees(found.stars[0].dir, skyDirection(s.x, s.y, W, H)) < 0.25)
  // Without any disc there are none.
  assert.equal(findStars(clean).discs.length, 0)
  assert.equal(findStars(backdrop(W, H, { seed: 1 }), { threshold: 5 }).discs.length, 0, 'the band is not a disc')
}

// ---- A big picture is fast enough ----------------------------------------------

{
  const BW = 8192
  const BH = 4096
  const img = backdrop(BW, BH, { seed: 3 })
  const stars = makeStars(BW, BH, 3000, 99, { sigmaRange: [0.7, 1.2], peakRange: [60, 250] })
  for (const s of stars) paintStar(img, s)
  const t0 = performance.now()
  const result = findStars(img)
  const ms = performance.now() - t0
  console.log(`starCatalog: ${BW}x${BH}, ${result.stars.length} stars found of ${stars.length} drawn in ${(ms / 1000).toFixed(2)} s`)
  assert.ok(ms < 6000, `findStars took ${ms} ms`)
  assert.ok(result.stars.length >= stars.length * 0.9 && result.stars.length <= stars.length * 1.05)
}

assert.equal(typeof CATALOGUE_VERSION, 'number')
console.log('starCatalog: all tests passed')
