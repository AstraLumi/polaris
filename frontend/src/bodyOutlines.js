// Plain body outlines for the height chart: a front-view figure for a sex
// (male, female, or in between for anyone else), an age (child, teen, adult,
// elderly) and a build (slim to heavy). Pure: no drawing, only SVG path
// strings in a box where the figure is 1 tall (top of the head at y = 0, the
// soles at y = 1) and centred on x = 0, so the chart scales it to the
// character's height. A body type picks the outline's parts; what it leaves
// empty comes from the character. The keys exist twice, here and in
// backend/bodyoutlines.go: change both.

// Marks a label for translation (shown with $t); no import keeps the module
// loadable by the node tests.
const tr = (s) => s

export const OUTLINE_SEXES = [
  { key: 'male', label: tr('Male') },
  { key: 'female', label: tr('Female') },
]
export const OUTLINE_AGES = [
  { key: 'child', label: tr('Child') },
  { key: 'teen', label: tr('Teen') },
  { key: 'adult', label: tr('Adult') },
  { key: 'elderly', label: tr('Elderly') },
]
export const OUTLINE_BUILDS = [
  { key: 'slim', label: tr('Slim') },
  { key: 'average', label: tr('Average') },
  { key: 'athletic', label: tr('Athletic') },
  { key: 'chubby', label: tr('Chubby') },
  { key: 'heavy', label: tr('Heavy') },
]

// The age a figure is drawn at, from the character's age in years.
export function ageStage(years) {
  if (years == null || !Number.isFinite(years)) return 'adult'
  if (years < 13) return 'child'
  if (years < 18) return 'teen'
  if (years < 60) return 'adult'
  return 'elderly'
}

// 'male', 'female' or 'neutral' (unknown or typed genders) from a gender.
export function sexOf(gender) {
  const g = String(gender || '').trim().toLowerCase()
  return g === 'male' ? 'male' : g === 'female' ? 'female' : 'neutral'
}

// The outline a version is drawn with: the body type's choices, the rest
// from the character.
export function outlineFor(bodyOutline, { gender, age } = {}) {
  const o = bodyOutline || {}
  return {
    sex: o.sex || sexOf(gender),
    age: o.age || ageStage(age),
    build: o.build || 'average',
  }
}

// Proportions, as fractions of the height: y down from the top of the head,
// x half-widths from the middle.
const ADULT_MALE = {
  chin: 0.133, headHalf: 0.045, neckHalf: 0.026,
  shoulderY: 0.183, shoulderHalf: 0.128,
  armpitY: 0.245, chestHalf: 0.102, bustY: 0.275, bustHalf: 0.1,
  waistY: 0.39, waistHalf: 0.08, hipY: 0.49, hipHalf: 0.096,
  crotchY: 0.53, kneeY: 0.72, kneeOuter: 0.066, kneeInner: 0.02,
  calfY: 0.8, calfOuter: 0.066, calfInner: 0.022,
  ankleY: 0.965, ankleOuter: 0.042, ankleInner: 0.024, footOuter: 0.062, footInner: 0.02,
  elbowY: 0.38, wristY: 0.505, fingerY: 0.615,
  upperArm: 0.022, elbowHalf: 0.017, forearm: 0.019, wristHalf: 0.013, handHalf: 0.017,
  shoulderDrop: 0.012,
}
const ADULT_FEMALE = {
  ...ADULT_MALE,
  chin: 0.137, headHalf: 0.046, neckHalf: 0.022,
  shoulderY: 0.185, shoulderHalf: 0.115,
  armpitY: 0.245, chestHalf: 0.092, bustY: 0.285, bustHalf: 0.1,
  waistY: 0.38, waistHalf: 0.068, hipY: 0.5, hipHalf: 0.109,
  kneeOuter: 0.062, kneeInner: 0.017, calfOuter: 0.062, calfInner: 0.02,
  ankleOuter: 0.037, ankleInner: 0.021, footOuter: 0.052, footInner: 0.018,
  elbowY: 0.375, wristY: 0.5, fingerY: 0.605,
  upperArm: 0.019, elbowHalf: 0.015, forearm: 0.016, wristHalf: 0.011, handHalf: 0.015,
  shoulderDrop: 0.016,
}
const CHILD = {
  ...ADULT_MALE,
  chin: 0.18, headHalf: 0.064, neckHalf: 0.028,
  shoulderY: 0.222, shoulderHalf: 0.112,
  armpitY: 0.28, chestHalf: 0.095, bustY: 0.31, bustHalf: 0.094,
  waistY: 0.42, waistHalf: 0.088, hipY: 0.52, hipHalf: 0.092,
  crotchY: 0.56, kneeY: 0.755, kneeOuter: 0.058, kneeInner: 0.02,
  calfY: 0.825, calfOuter: 0.058, calfInner: 0.022,
  ankleY: 0.962, ankleOuter: 0.04, ankleInner: 0.022, footOuter: 0.058, footInner: 0.02,
  elbowY: 0.41, wristY: 0.53, fingerY: 0.625,
  upperArm: 0.022, elbowHalf: 0.019, forearm: 0.019, wristHalf: 0.015, handHalf: 0.02,
  shoulderDrop: 0.016,
}

// How much each build widens (or narrows) each part.
const BUILDS = {
  slim: { head: 0.98, neck: 0.9, shoulder: 0.93, chest: 0.86, waist: 0.84, hip: 0.9, leg: 0.82, arm: 0.8 },
  average: { head: 1, neck: 1, shoulder: 1, chest: 1, waist: 1, hip: 1, leg: 1, arm: 1 },
  athletic: { head: 1, neck: 1.22, shoulder: 1.12, chest: 1.1, waist: 0.98, hip: 0.98, leg: 1.12, arm: 1.25 },
  chubby: { head: 1.05, neck: 1.18, shoulder: 1.05, chest: 1.15, waist: 1.32, hip: 1.17, leg: 1.25, arm: 1.28 },
  heavy: { head: 1.09, neck: 1.32, shoulder: 1.12, chest: 1.3, waist: 1.62, hip: 1.36, leg: 1.48, arm: 1.55 },
}

function mix(a, b, t) {
  const out = {}
  for (const k in a) out[k] = a[k] + (b[k] - a[k]) * t
  return out
}

// The proportions for one outline, before the build.
function proportions(sex, age) {
  const female = sex === 'female' ? 1 : sex === 'neutral' ? 0.5 : 0
  const adult = mix(ADULT_MALE, ADULT_FEMALE, female)
  if (age === 'child') return { ...CHILD, headHalf: CHILD.headHalf * (1 + 0.02 * female) }
  if (age === 'teen') {
    // Most of the way grown; hips and shoulders still filling out.
    const p = mix(CHILD, adult, 0.68)
    p.hipHalf = p.hipHalf - 0.006 * female
    p.bustHalf = Math.max(p.chestHalf, p.bustHalf - 0.004)
    return p
  }
  if (age === 'elderly') {
    return {
      ...adult,
      // Hunched: the shoulders rise to the head and round off, the middle
      // thickens, the arms bend a little.
      shoulderY: adult.shoulderY - 0.006,
      shoulderHalf: adult.shoulderHalf * 0.95,
      shoulderDrop: adult.shoulderDrop + 0.01,
      bustY: adult.bustY + 0.015 * female,
      waistHalf: adult.waistHalf * 1.12,
      hipHalf: adult.hipHalf * 1.02,
      elbowBend: 0.008,
    }
  }
  return adult
}

// A smooth closed path through points; a point with `corner` keeps its
// corner.
function smoothPath(points) {
  const n = points.length
  const f = (v) => +v.toFixed(5)
  const tangent = (i) => {
    const p = points[i]
    if (p.corner) return [0, 0]
    const a = points[(i - 1 + n) % n]
    const b = points[(i + 1) % n]
    return [(b.x - a.x) / 6, (b.y - a.y) / 6]
  }
  let d = `M${f(points[0].x)} ${f(points[0].y)}`
  for (let i = 0; i < n; i++) {
    const p = points[i]
    const q = points[(i + 1) % n]
    const tp = tangent(i)
    const tq = tangent((i + 1) % n)
    d += `C${f(p.x + tp[0])} ${f(p.y + tp[1])} ${f(q.x - tq[0])} ${f(q.y - tq[1])} ${f(q.x)} ${f(q.y)}`
  }
  return d + 'Z'
}

// The right side (top to bottom, ending in the middle) and its mirror.
function mirrored(right) {
  const left = right
    .filter((p) => p.x > 1e-9)
    .reverse()
    .map((p) => ({ ...p, x: -p.x }))
  return smoothPath([...right, ...left])
}

function ellipse(cx, cy, rx, ry) {
  const f = (v) => +v.toFixed(5)
  return `M${f(cx - rx)} ${f(cy)}A${f(rx)} ${f(ry)} 0 1 0 ${f(cx + rx)} ${f(cy)}A${f(rx)} ${f(ry)} 0 1 0 ${f(cx - rx)} ${f(cy)}Z`
}

// An arm hanging from the shoulder: a tapered tube round a centre line.
function arm(centre, side) {
  const out = []
  const back = []
  for (let i = 0; i < centre.length; i++) {
    const c = centre[i]
    const a = centre[Math.max(0, i - 1)]
    const b = centre[Math.min(centre.length - 1, i + 1)]
    const dx = b.x - a.x
    const dy = b.y - a.y
    const len = Math.hypot(dx, dy) || 1
    // The normal pointing away from the body.
    const nx = dy / len
    const ny = -dx / len
    out.push({ x: (c.x + nx * c.w) * side, y: c.y + ny * c.w })
    back.push({ x: (c.x - nx * c.w) * side, y: c.y - ny * c.w })
  }
  const end = centre[centre.length - 1]
  return smoothPath([...out, { x: end.x * side, y: end.y + end.w * 0.6 }, ...back.reverse()])
}

// The figure for an outline: { shapes: [path], halfWidth }, 1 tall.
export function figure({ sex = 'neutral', age = 'adult', build = 'average' } = {}) {
  const p = proportions(sex, age)
  const b = BUILDS[build] || BUILDS.average
  const female = sex === 'female' ? 1 : sex === 'neutral' ? 0.5 : 0

  const headHalf = p.headHalf * b.head
  const headRy = p.chin / 2
  const neck = p.neckHalf * b.neck
  const shoulder = p.shoulderHalf * b.shoulder
  const chest = p.chestHalf * b.chest
  const bust = Math.max(chest, p.bustHalf * b.chest)
  const waist = p.waistHalf * b.waist
  const hip = p.hipHalf * b.hip
  const leg = b.leg
  const legGap = build === 'heavy' || build === 'chubby' ? 0.002 : 0.006
  const thighOuter = Math.max(hip * 0.97, waist * 0.92)

  const body = mirrored([
    { x: 0, y: p.chin - 0.01 },
    { x: neck, y: p.chin - 0.012 },
    { x: neck * 1.08, y: p.shoulderY - p.shoulderDrop * 0.9 },
    { x: neck + (shoulder - neck) * 0.55, y: p.shoulderY - p.shoulderDrop * 0.35 },
    { x: shoulder, y: p.shoulderY + 0.022 },
    { x: Math.max(chest, shoulder * 0.86), y: p.armpitY },
    { x: bust, y: p.bustY },
    { x: waist, y: p.waistY },
    { x: hip, y: p.hipY },
    { x: thighOuter, y: p.crotchY + 0.04 },
    { x: Math.max(p.kneeOuter * leg, p.kneeInner + 0.03), y: p.kneeY },
    { x: Math.max(p.calfOuter * leg, p.calfInner + 0.03), y: p.calfY },
    { x: p.ankleOuter * Math.sqrt(leg) + 0.004, y: p.ankleY },
    { x: p.footOuter * Math.sqrt(leg), y: 1, corner: true },
    { x: p.footInner, y: 1, corner: true },
    { x: p.ankleInner, y: p.ankleY },
    { x: p.calfInner, y: p.calfY },
    { x: p.kneeInner, y: p.kneeY },
    { x: legGap, y: p.crotchY + 0.03 },
    { x: 0, y: p.crotchY, corner: true },
  ])

  // Arms keep clear of the body however wide it gets.
  const a = b.arm
  const clear = (half, y) => {
    const torso = y < p.waistY ? Math.max(bust, waist) : Math.max(waist, hip)
    return torso + half + 0.004
  }
  const bend = p.elbowBend || 0
  const shoulderX = shoulder - p.upperArm * a * 0.7
  const elbowX = Math.max(shoulderX + 0.012 + bend, clear(p.elbowHalf * a, p.elbowY))
  const wristX = Math.max(elbowX + 0.006 - bend * 0.5, clear(p.wristHalf * a, p.wristY))
  const centre = [
    { x: shoulderX, y: p.shoulderY + 0.028, w: p.upperArm * a * 1.05 },
    { x: (shoulderX + elbowX) / 2, y: (p.shoulderY + 0.028 + p.elbowY) / 2, w: p.upperArm * a },
    { x: elbowX, y: p.elbowY, w: p.elbowHalf * a },
    { x: (elbowX + wristX) / 2, y: (p.elbowY + p.wristY) / 2, w: p.forearm * a },
    { x: wristX, y: p.wristY, w: p.wristHalf * Math.sqrt(a) },
    { x: wristX + 0.002, y: (p.wristY + p.fingerY) / 2, w: p.handHalf * Math.sqrt(a) },
    { x: wristX + 0.002, y: p.fingerY - p.handHalf * 0.6, w: p.handHalf * 0.75 * Math.sqrt(a) },
  ]

  const shapes = [ellipse(0, headRy, headHalf, headRy), body, arm(centre, 1), arm(centre, -1)]
  shapes.push(...hair(sex, age, headHalf, headRy, p, female))
  return { shapes, halfWidth: Math.max(wristX + p.handHalf * a, p.footOuter * leg) }
}

// Hair, low detail: short for men, long for women, pigtails for girls, a
// bun for old women; in between for anyone else.
function hair(sex, age, headHalf, headRy, p, female) {
  if (sex === 'male') {
    if (age === 'elderly') return []
    return [ellipse(0, headRy * 0.82, headHalf * 1.06, headRy * 0.86)]
  }
  if (sex === 'neutral') return [ellipse(0, headRy * 0.95, headHalf * 1.1, headRy * 1.0)]
  const cap = ellipse(0, headRy * 0.85, headHalf * 1.08, headRy * 0.9)
  if (age === 'child') {
    return [cap, ellipse(headHalf * 1.18, headRy * 1.05, 0.018, 0.032), ellipse(-headHalf * 1.18, headRy * 1.05, 0.018, 0.032)]
  }
  if (age === 'elderly') return [cap, ellipse(0, 0.004, headHalf * 0.42, headHalf * 0.36)]
  // Long hair, falling past the shoulders behind the back.
  const end = p.shoulderY + 0.07
  const long = mirrored([
    { x: 0, y: -0.004 },
    { x: headHalf * 0.92, y: headRy * 0.25 },
    { x: headHalf * 1.2, y: headRy * 1.1 },
    { x: headHalf * 1.38, y: p.chin + 0.015 },
    { x: headHalf * 1.55, y: end, corner: true },
    { x: 0, y: end + 0.01, corner: true },
  ])
  return [long]
}
