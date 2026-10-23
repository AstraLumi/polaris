// The sky view's drawing (SkyView.vue): the sky picture's glow, the stars
// as points, the bodies as lit spheres or points, clouds, the ground. All in
// the local horizon frame, three.js's way round: x east, y up, z south (so
// -z is north). Only directions come here (skyFrames.js works out the
// rest); nothing is drawn at its distance. Only lazily loaded pages import
// this.
import * as THREE from 'three'

// The local [east, north, up] of skyFrames.js as a three.js vector.
export const toThree = (v) => new THREE.Vector3(v[0], v[2], -v[1])

// The rows of starMatrix (picture frame → local) as a matrix from the
// picture's frame to three.js's.
export function pictureToThree(m) {
  const out = new THREE.Matrix3()
  out.set(m[0][0], m[0][1], m[0][2], m[2][0], m[2][1], m[2][2], -m[1][0], -m[1][1], -m[1][2])
  return out
}

// Clouds: a deck seen from below, thin noise laid on a plane overhead, so
// they foreshorten towards the horizon. uCover is how much of the sky they
// hide (the climate's cloud cover there that month). uDip, here and below,
// is the sine of how far the real horizon lies under the level: seen from
// high up, the planet curves away and more of the sky shows.
const CLOUD_GLSL = `
uniform float uCover;
uniform float uDip;
uniform vec2 uCloudShift;
float cHash(vec3 p) {
  p = fract(p * 0.3183099 + 0.1);
  p *= 17.0;
  return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
}
float cNoise(vec3 x) {
  vec3 i = floor(x);
  vec3 f = fract(x);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(mix(cHash(i), cHash(i + vec3(1, 0, 0)), f.x), mix(cHash(i + vec3(0, 1, 0)), cHash(i + vec3(1, 1, 0)), f.x), f.y),
             mix(mix(cHash(i + vec3(0, 0, 1)), cHash(i + vec3(1, 0, 1)), f.x), mix(cHash(i + vec3(0, 1, 1)), cHash(i + vec3(1, 1, 1)), f.x), f.y), f.z);
}
float cloudAt(vec3 d) {
  float y = d.y + uDip;
  if (uCover <= 0.001 || y <= 0.0) return 0.0;
  vec2 q = d.xz / (y + 0.06) * 1.4 + uCloudShift;
  float n = cNoise(vec3(q, 0.0)) * 0.5 + cNoise(vec3(q * 2.3, 1.7)) * 0.3 + cNoise(vec3(q * 5.1, 3.1)) * 0.2;
  float t = n + (uCover - 0.5) * 0.95;
  return smoothstep(0.32, 0.62, t) * min(1.0, 0.25 + uCover * 1.5);
}`

// ---- The sky behind everything -------------------------------------------------------------------
const DOME_VERTEX = `
varying vec3 vDir;
void main() {
  vDir = position;
  vec4 p = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  gl_Position = p.xyww;
}`

const DOME_FRAGMENT = `
uniform sampler2D uSky;
uniform mat3 uToPicture;
uniform float uGain;
uniform float uDay;
uniform vec3 uAir;
uniform vec3 uSunDir;
uniform float uShell;
uniform vec3 uShellColor;
uniform float uDip;
varying vec3 vDir;
void main() {
  vec3 d = normalize(vDir);
  vec3 p = uToPicture * d;
  float lon = atan(p.y, p.x);
  float lat = asin(clamp(p.z, -1.0, 1.0));
  vec3 sky = texture2D(uSky, vec2(0.5 - lon / 6.283185307, 0.5 + lat / 3.141592654)).rgb * uGain;
  // A shell sky glows by its own day, its painted stars fading.
  sky = mix(sky, uShellColor * 0.8 + sky * 0.2, uShell * 0.9);
  // Daylight in the air: brighter low down and round the sun.
  float low = 1.0 - clamp(d.y + uDip, 0.0, 1.0);
  float toward = max(dot(d, uSunDir), 0.0);
  vec3 air = uAir * (0.6 + 0.5 * low) + vec3(1.0, 0.86, 0.65) * pow(toward, 6.0) * 0.55;
  gl_FragColor = vec4(mix(sky, air, uDay), 1.0);
}`

export function createSkyDome(texture) {
  const mesh = new THREE.Mesh(
    new THREE.SphereGeometry(1, 64, 32),
    new THREE.ShaderMaterial({
      vertexShader: DOME_VERTEX,
      fragmentShader: DOME_FRAGMENT,
      uniforms: {
        uSky: { value: texture },
        uToPicture: { value: new THREE.Matrix3() },
        uGain: { value: 1 },
        uDay: { value: 0 },
        uAir: { value: new THREE.Color(0.32, 0.55, 0.95) },
        uSunDir: { value: new THREE.Vector3(0, -1, 0) },
        uShell: { value: 0 },
        uShellColor: { value: new THREE.Color(1, 0.95, 0.88) },
        uDip: { value: 0 },
      },
      side: THREE.BackSide,
      depthWrite: false,
      depthTest: false,
    }),
  )
  mesh.renderOrder = -3
  mesh.frustumCulled = false
  return mesh
}

// ---- Points: stars and far bodies --------------------------------------------------------------------
// Each point a direction (in the picture's frame for stars, turned by
// uFromPicture; the identity for bodies, already local), a magnitude and a
// colour. uLimit is the faintest magnitude seen now (darkness, zoom); the
// brighter, the bigger and more opaque. Hidden below the horizon and
// behind clouds.
const POINT_VERTEX = `
attribute float aMag;
attribute vec3 aColor;
uniform mat3 uFromPicture;
uniform float uLimit;
uniform float uPx;
uniform float uVisible;
varying vec3 vColor;
varying float vAlpha;
${CLOUD_GLSL}
void main() {
  vec3 d = uFromPicture * position;
  float bright = uLimit - aMag;
  vAlpha = clamp(bright / 2.2, 0.0, 1.0) * uVisible * smoothstep(-0.004, 0.08, d.y + uDip) * (1.0 - cloudAt(d));
  gl_PointSize = clamp(1.4 + bright * 0.6, 1.0, 10.0) * uPx;
  vColor = aColor;
  gl_Position = projectionMatrix * viewMatrix * vec4(d * 500.0, 1.0);
}`

const POINT_FRAGMENT = `
varying vec3 vColor;
varying float vAlpha;
void main() {
  float r = length(gl_PointCoord - 0.5) * 2.0;
  float a = vAlpha * (1.0 - smoothstep(0.15, 1.0, r));
  if (a < 0.003) discard;
  gl_FragColor = vec4(vColor * a, a);
}`

// dirs: Float32Array (3 a point); mags: Float32Array; colors: Uint8Array (3 a point).
export function createPoints(dirs, mags, colors) {
  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(dirs, 3))
  geo.setAttribute('aMag', new THREE.BufferAttribute(mags, 1))
  geo.setAttribute('aColor', new THREE.BufferAttribute(colors, 3, true))
  const points = new THREE.Points(
    geo,
    new THREE.ShaderMaterial({
      vertexShader: POINT_VERTEX,
      fragmentShader: POINT_FRAGMENT,
      uniforms: {
        uFromPicture: { value: new THREE.Matrix3() },
        uLimit: { value: 6 },
        uPx: { value: 1 },
        uVisible: { value: 1 },
        uCover: { value: 0 },
        uCloudShift: { value: new THREE.Vector2() },
        uDip: { value: 0 },
      },
      transparent: true,
      depthWrite: false,
      depthTest: false,
      blending: THREE.AdditiveBlending,
    }),
  )
  points.frustumCulled = false
  points.renderOrder = -2
  return points
}

// ---- Bodies as spheres -------------------------------------------------------------------------------
// A moon or planet with a visible disc: a sphere a set distance away, its
// size its true angular size, lit from the star's side (uDirect, phases)
// and from all round (uEven: a shell, its painted starlight).
const BODY_VERTEX = `
varying vec3 vN;
void main() {
  vN = normalize(normal);
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`

const BODY_FRAGMENT = `
uniform vec3 uColor;
uniform vec3 uLight;
uniform float uDirect;
uniform float uEven;
uniform float uDay;
varying vec3 vN;
void main() {
  float lit = uDirect * max(dot(normalize(vN), uLight), 0.0) + uEven;
  // The eye takes a lit moon as near white, its colour a tint.
  vec3 c = mix(uColor, vec3(1.0), 0.55) * (1.0 - exp(-4.0 * lit)) * 1.1;
  // By day the sky's light shows through a dark limb.
  gl_FragColor = vec4(c, max(0.0, 1.0 - uDay * (1.0 - min(1.0, lit * 2.0))));
}`

export const BODY_DISTANCE = 100

export function createBody(color) {
  const mesh = new THREE.Mesh(
    new THREE.SphereGeometry(1, 48, 24),
    new THREE.ShaderMaterial({
      vertexShader: BODY_VERTEX,
      fragmentShader: BODY_FRAGMENT,
      uniforms: {
        uColor: { value: new THREE.Color(color) },
        uLight: { value: new THREE.Vector3(0, 1, 0) },
        uDirect: { value: 1 },
        uEven: { value: 0 },
        uDay: { value: 0 },
      },
      transparent: true,
      depthWrite: false,
    }),
  )
  mesh.renderOrder = -1
  return mesh
}

// The star: a bright disc and a glow round it.
export function createSun(color) {
  const group = new THREE.Group()
  const disc = new THREE.Mesh(new THREE.SphereGeometry(1, 32, 16), new THREE.MeshBasicMaterial({ color: new THREE.Color(color).lerp(new THREE.Color('#ffffff'), 0.6), depthWrite: false }))
  disc.renderOrder = -1
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = 128
  const ctx = canvas.getContext('2d')
  const g = ctx.createRadialGradient(64, 64, 0, 64, 64, 64)
  g.addColorStop(0, 'rgba(255,255,255,0.9)')
  g.addColorStop(0.15, 'rgba(255,240,210,0.45)')
  g.addColorStop(1, 'rgba(255,220,160,0)')
  ctx.fillStyle = g
  ctx.fillRect(0, 0, 128, 128)
  const glare = new THREE.Sprite(new THREE.SpriteMaterial({ map: new THREE.CanvasTexture(canvas), color: new THREE.Color(color).lerp(new THREE.Color('#ffffff'), 0.5), blending: THREE.AdditiveBlending, depthWrite: false, depthTest: false }))
  glare.renderOrder = 0
  group.add(disc, glare)
  return { group, disc, glare }
}

// ---- Clouds overhead ------------------------------------------------------------------------------------
const CLOUD_FRAGMENT = `
uniform float uDay;
uniform float uMoon;
uniform vec3 uTint;
varying vec3 vDir;
${CLOUD_GLSL}
void main() {
  vec3 d = normalize(vDir);
  float a = cloudAt(d);
  if (a < 0.003) discard;
  vec3 night = vec3(0.025, 0.03, 0.045) + vec3(0.16, 0.17, 0.2) * uMoon;
  vec3 day = mix(vec3(0.72, 0.74, 0.78), vec3(0.95), clamp(d.y + uDip, 0.0, 1.0)) * uTint;
  gl_FragColor = vec4(mix(night, day, uDay), a * 0.97);
}`

export function createClouds() {
  const mesh = new THREE.Mesh(
    new THREE.SphereGeometry(2, 64, 32),
    new THREE.ShaderMaterial({
      vertexShader: DOME_VERTEX,
      fragmentShader: CLOUD_FRAGMENT,
      uniforms: { uCover: { value: 0 }, uCloudShift: { value: new THREE.Vector2() }, uDip: { value: 0 }, uDay: { value: 0 }, uMoon: { value: 0 }, uTint: { value: new THREE.Color(1, 1, 1) } },
      side: THREE.BackSide,
      transparent: true,
      depthWrite: false,
      depthTest: false,
    }),
  )
  mesh.renderOrder = 1
  mesh.frustumCulled = false
  return mesh
}

// ---- The ground ---------------------------------------------------------------------------------------
// The ground runs out to the real horizon, uDip under the level; uGround is
// the colour of what covers the hex.
const GROUND_FRAGMENT = `
uniform float uDay;
uniform vec3 uGround;
uniform float uDip;
varying vec3 vDir;
void main() {
  vec3 d = normalize(vDir);
  float y = d.y + uDip;
  // Its edge smoothed over a pixel, so a tilted horizon doesn't step.
  float a = clamp(-y / max(fwidth(y), 1e-6), 0.0, 1.0);
  if (a <= 0.0) discard;
  float edge = smoothstep(-0.02, 0.0, y);
  vec3 c = mix(vec3(0.012, 0.014, 0.02), uGround, uDay) * (0.55 + 0.45 * smoothstep(-0.5, 0.0, y));
  c += vec3(0.08, 0.09, 0.12) * edge * (0.3 + uDay);
  gl_FragColor = vec4(c, a);
}`

export function createGround() {
  const mesh = new THREE.Mesh(
    new THREE.SphereGeometry(3, 64, 32),
    new THREE.ShaderMaterial({
      vertexShader: DOME_VERTEX,
      fragmentShader: GROUND_FRAGMENT,
      uniforms: { uDay: { value: 0 }, uGround: { value: new THREE.Color(0.2, 0.22, 0.16) }, uDip: { value: 0 } },
      side: THREE.BackSide,
      // Transparent only to be drawn last, over all else below the horizon.
      transparent: true,
      depthWrite: false,
      depthTest: false,
    }),
  )
  mesh.renderOrder = 2
  mesh.frustumCulled = false
  return mesh
}

// ---- Altitude and azimuth lines ---------------------------------------------------------------------------
export function createGrid() {
  const pts = []
  const dir = (az, alt) => new THREE.Vector3(Math.sin(az) * Math.cos(alt), Math.sin(alt), -Math.cos(az) * Math.cos(alt)).multiplyScalar(400)
  const D = Math.PI / 180
  for (let alt = 0; alt < 90; alt += 15) {
    for (let az = 0; az < 360; az += 3) pts.push(dir(az * D, alt * D), dir((az + 3) * D, alt * D))
  }
  for (let az = 0; az < 360; az += 30) {
    for (let alt = 0; alt < 87; alt += 3) pts.push(dir(az * D, alt * D), dir(az * D, (alt + 3) * D))
  }
  const lines = new THREE.LineSegments(new THREE.BufferGeometry().setFromPoints(pts), new THREE.LineBasicMaterial({ color: 0x6f86b8, transparent: true, opacity: 0.28, depthTest: false, depthWrite: false }))
  lines.renderOrder = 3
  lines.frustumCulled = false
  return lines
}
