// How the solar system's bodies look (SolarScene.vue): stars, black holes
// and Dyson spheres; planets lit by their star with a soft line between day
// and night, their air, clouds and rings. uPower is how much light the star
// gives (0 inside a Dyson sphere). Only lazily loaded pages import this
// (three.js).
import * as THREE from 'three'

// ---- Stars ------------------------------------------------------------------------------

// A little value noise, for the star's grainy face.
const NOISE = `
float hash(vec3 p) {
  p = fract(p * 0.3183099 + 0.1);
  p *= 17.0;
  return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
}
float noise(vec3 x) {
  vec3 i = floor(x);
  vec3 f = fract(x);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(mix(hash(i), hash(i + vec3(1, 0, 0)), f.x), mix(hash(i + vec3(0, 1, 0)), hash(i + vec3(1, 1, 0)), f.x), f.y),
             mix(mix(hash(i + vec3(0, 0, 1)), hash(i + vec3(1, 0, 1)), f.x), mix(hash(i + vec3(0, 1, 1)), hash(i + vec3(1, 1, 1)), f.x), f.y), f.z);
}`

const STAR_VERTEX = `
varying vec3 vNormal;
varying vec3 vView;
varying vec3 vLocal;
void main() {
  vec4 world = modelMatrix * vec4(position, 1.0);
  vNormal = normalize(mat3(modelMatrix) * normal);
  vView = normalize(cameraPosition - world.xyz);
  vLocal = normalize(position);
  gl_Position = projectionMatrix * viewMatrix * world;
}`

// The star's face. A star (uMode 0): nearly white in the middle, its own
// colour towards the edge and darker and redder at the very rim (limb
// darkening), with a faint boiling grain. A black hole (1): black. A Dyson
// sphere (2): a dark shell with a faint warm lattice, the waste heat of
// whatever is inside.
const STAR_FRAGMENT = `
uniform vec3 uColor;
uniform float uMode;
uniform float uTime;
varying vec3 vNormal;
varying vec3 vView;
varying vec3 vLocal;
${NOISE}
void main() {
  float mu = clamp(dot(normalize(vNormal), normalize(vView)), 0.0, 1.0);
  if (uMode > 1.5) {
    float lat = asin(clamp(vLocal.z, -1.0, 1.0));
    float lon = atan(vLocal.y, vLocal.x);
    vec2 g = abs(fract(vec2(lat * 2.546, lon * 1.910)) - 0.5);
    float line = 1.0 - smoothstep(0.0, 0.035, min(g.x, g.y));
    gl_FragColor = vec4(vec3(0.02, 0.018, 0.025) + vec3(1.0, 0.42, 0.14) * line * (0.12 + 0.25 * mu), 1.0);
    return;
  }
  if (uMode > 0.5) {
    gl_FragColor = vec4(0.0, 0.0, 0.0, 1.0);
    return;
  }
  float grain = noise(vLocal * 26.0 + uTime * 0.04) * 0.6 + noise(vLocal * 63.0 - uTime * 0.07) * 0.4;
  vec3 core = mix(uColor, vec3(1.0), 0.6);
  vec3 rim = uColor * vec3(1.0, 0.68, 0.48) * 0.6;
  vec3 face = mix(rim, core, pow(mu, 0.55)) * (0.92 + 0.16 * grain);
  gl_FragColor = vec4(face, 1.0);
}`

// The light round it: a quad always facing the camera. A star: a soft glow
// falling off smoothly from under its edge, with barely-there rays, and a
// wide faint halo. A black hole: a thin bright ring of light bent round it
// (the photon ring) in a dim glow. A Dyson sphere: only a faint warm haze.
const CORONA_VERTEX = `
varying vec2 vUv;
void main() {
  vUv = uv * 2.0 - 1.0;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`

const CORONA_FRAGMENT = `
uniform vec3 uColor;
uniform float uTime;
uniform float uCore;   // the star's radius, as a part of the quad's half-size
uniform float uMode;   // 0 star, 1 black hole, 2 Dyson sphere
varying vec2 vUv;
void main() {
  float d = length(vUv);
  if (d > 1.0) discard;
  float a = atan(vUv.y, vUv.x);
  float x = max(d - uCore * 0.92, 0.0) / (1.0 - uCore * 0.92);
  float rays = 0.94 + 0.06 * sin(a * 7.0 + uTime * 0.3) * sin(a * 4.0 - uTime * 0.17);
  float edge = 1.0 - smoothstep(0.85, 1.0, d);
  float k;
  if (uMode > 1.5) {
    k = exp(-x * 7.0) * 0.1;
  } else if (uMode > 0.5) {
    float ring = exp(-pow((d - uCore * 1.5) / (uCore * 0.12), 2.0));
    k = ring * 1.1 + exp(-x * 4.0) * 0.25;
    if (d < uCore * 1.3) k *= smoothstep(uCore * 1.05, uCore * 1.3, d);
  } else {
    k = (exp(-x * 6.0) * 0.95 * rays + exp(-x * 1.8) * 0.16) * edge;
  }
  gl_FragColor = vec4(uColor * k, k);
}`

// A star of radius r: { group, faceCamera(camera, time) }. blackHole and
// dyson change how it looks (and a Dyson sphere gives no light: the scene
// turns its planets' light off).
export function createStar(r, color, { blackHole = false, dyson = false } = {}) {
  const group = new THREE.Group()
  const c = new THREE.Color(color)
  const mode = dyson ? 2 : blackHole ? 1 : 0
  const face = new THREE.Mesh(
    new THREE.SphereGeometry(r, 64, 32),
    new THREE.ShaderMaterial({
      vertexShader: STAR_VERTEX,
      fragmentShader: STAR_FRAGMENT,
      uniforms: { uColor: { value: c }, uMode: { value: mode }, uTime: { value: 0 } },
    }),
  )
  const size = r * (blackHole ? 3.4 : 6)
  const corona = new THREE.Mesh(
    new THREE.PlaneGeometry(size * 2, size * 2),
    new THREE.ShaderMaterial({
      vertexShader: CORONA_VERTEX,
      fragmentShader: CORONA_FRAGMENT,
      uniforms: {
        uColor: { value: blackHole ? new THREE.Color('#ffb066') : dyson ? new THREE.Color('#ff6a2a') : c },
        uTime: { value: 0 },
        uCore: { value: r / size },
        uMode: { value: mode },
      },
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    }),
  )
  corona.userData.corona = true
  group.add(face, corona)
  return {
    group,
    faceCamera(camera, time) {
      corona.quaternion.copy(camera.quaternion)
      corona.material.uniforms.uTime.value = time
      face.material.uniforms.uTime.value = time
    },
  }
}

// ---- Planets ----------------------------------------------------------------------------

// A plain planet or moon: its colour lit by the star at uSun (in the
// world), with a soft twilight band instead of a hard edge, and a faint
// glow left on the night side. A gas giant (uBands) is striped along its
// latitudes, which follow its spin axis (the sphere's own z).
const PLANET_VERTEX = `
varying vec3 vNormal;
varying vec3 vWorld;
varying vec3 vLocal;
void main() {
  vec4 world = modelMatrix * vec4(position, 1.0);
  vWorld = world.xyz;
  vLocal = normalize(position);
  vNormal = normalize(mat3(modelMatrix) * normal);
  gl_Position = projectionMatrix * viewMatrix * world;
}`

const PLANET_FRAGMENT = `
uniform vec3 uColor;
uniform vec3 uSun;
uniform float uPower;
uniform float uBands;
varying vec3 vNormal;
varying vec3 vWorld;
varying vec3 vLocal;
void main() {
  vec3 base = uColor;
  if (uBands > 0.5) {
    float lon = atan(vLocal.y, vLocal.x);
    float wide = sin(vLocal.z * 22.0 + sin(vLocal.z * 9.0 + lon * 2.0) * 0.6) * 0.5 + 0.5;
    float fine = sin(vLocal.z * 51.0 + sin(lon * 3.0 + vLocal.z * 14.0) * 0.4) * 0.5 + 0.5;
    base = uColor * mix(0.7, 1.18, wide * 0.7 + fine * 0.3);
  }
  float d = dot(normalize(vNormal), normalize(uSun - vWorld));
  float day = smoothstep(-0.12, 0.35, d) * uPower;
  vec3 night = base * 0.05 + vec3(0.01, 0.015, 0.03);
  gl_FragColor = vec4(mix(night, base * (0.25 + 0.85 * max(d, 0.0)), day), 1.0);
}`

export function createPlanetMaterial(color, { bands = false } = {}) {
  return new THREE.ShaderMaterial({
    vertexShader: PLANET_VERTEX,
    fragmentShader: PLANET_FRAGMENT,
    uniforms: { uColor: { value: new THREE.Color(color) }, uSun: { value: new THREE.Vector3() }, uPower: { value: 1 }, uBands: { value: bands ? 1 : 0 } },
  })
}

// ---- Atmospheres ----------------------------------------------------------------------
// A thin glow round the planet's edge (a shell a little larger than it,
// bright where it is seen edge-on), stronger on the side the star lights.
const ATMOSPHERE_FRAGMENT = `
uniform vec3 uColor;
uniform vec3 uSun;
uniform float uPower;
varying vec3 vNormal;
varying vec3 vWorld;
void main() {
  vec3 n = normalize(vNormal);
  float rim = pow(1.0 - max(dot(n, normalize(cameraPosition - vWorld)), 0.0), 3.0);
  float lit = smoothstep(-0.3, 0.45, dot(n, normalize(uSun - vWorld)));
  gl_FragColor = vec4(uColor * rim * (0.04 + 0.08 * uPower + 1.1 * lit * uPower), 1.0);
}`

export function createAtmosphere(r, color) {
  return new THREE.Mesh(
    new THREE.SphereGeometry(r * 1.06, 48, 24),
    new THREE.ShaderMaterial({
      vertexShader: PLANET_VERTEX,
      fragmentShader: ATMOSPHERE_FRAGMENT,
      uniforms: { uColor: { value: new THREE.Color(color) }, uSun: { value: new THREE.Vector3() }, uPower: { value: 1 } },
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    }),
  )
}

// ---- Clouds -----------------------------------------------------------------------------
// A shell just above the surface. How cloudy each point is comes in with
// the geometry (aCover, from climate.js); the shader adds fine, drifting
// detail and lights them like the planet (dark on the night side).
const CLOUD_VERTEX = `
attribute float aCover;
varying vec3 vNormal;
varying vec3 vWorld;
varying vec3 vLocal;
varying float vCover;
void main() {
  vec4 world = modelMatrix * vec4(position, 1.0);
  vWorld = world.xyz;
  vLocal = normalize(position);
  vCover = aCover;
  vNormal = normalize(mat3(modelMatrix) * normal);
  gl_Position = projectionMatrix * viewMatrix * world;
}`

const CLOUD_FRAGMENT = `
uniform vec3 uSun;
uniform float uTime;
uniform float uPower;
varying vec3 vNormal;
varying vec3 vWorld;
varying vec3 vLocal;
varying float vCover;
float hash(vec3 p) {
  p = fract(p * 0.3183099 + 0.1);
  p *= 17.0;
  return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
}
float noise(vec3 x) {
  vec3 i = floor(x);
  vec3 f = fract(x);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(mix(hash(i), hash(i + vec3(1, 0, 0)), f.x), mix(hash(i + vec3(0, 1, 0)), hash(i + vec3(1, 1, 0)), f.x), f.y),
             mix(mix(hash(i + vec3(0, 0, 1)), hash(i + vec3(1, 0, 1)), f.x), mix(hash(i + vec3(0, 1, 1)), hash(i + vec3(1, 1, 1)), f.x), f.y), f.z);
}
void main() {
  vec3 p = vLocal * 7.0 + vec3(uTime, uTime * 0.4, 0.0);
  float detail = noise(p) * 0.5 + noise(p * 2.3) * 0.3 + noise(p * 5.1) * 0.2;
  float a = smoothstep(0.3, 0.7, vCover * 0.9 + (detail - 0.5) * 0.55);
  float d = dot(normalize(vNormal), normalize(uSun - vWorld));
  float day = smoothstep(-0.15, 0.3, d) * uPower;
  vec3 c = mix(vec3(0.02, 0.03, 0.06), vec3(0.6 + 0.4 * max(d, 0.0)), day);
  gl_FragColor = vec4(c, a * 0.88);
}`

// Clouds round a body of radius r: coverOf(direction on the unit sphere)
// says how cloudy it is there (0 to 1).
export function createClouds(r, coverOf) {
  const geo = new THREE.SphereGeometry(r * 1.02, 96, 48)
  const pos = geo.attributes.position
  const cover = new Float32Array(pos.count)
  const v = new THREE.Vector3()
  for (let i = 0; i < pos.count; i++) {
    v.fromBufferAttribute(pos, i).normalize()
    cover[i] = coverOf([v.x, v.y, v.z])
  }
  geo.setAttribute('aCover', new THREE.BufferAttribute(cover, 1))
  return new THREE.Mesh(
    geo,
    new THREE.ShaderMaterial({
      vertexShader: CLOUD_VERTEX,
      fragmentShader: CLOUD_FRAGMENT,
      uniforms: { uSun: { value: new THREE.Vector3() }, uTime: { value: 0 }, uPower: { value: 1 } },
      transparent: true,
      depthWrite: false,
    }),
  )
}

// ---- Rings ------------------------------------------------------------------------------
// A flat ring in the planet's equator (so it tilts with it), from inner to
// outer radius: fine bands of dust and ice with a dark gap, lit by the star
// from either side, and dark where the planet's shadow falls on it.
const RING_VERTEX = `
varying vec3 vWorld;
varying vec3 vLocal;
void main() {
  vec4 world = modelMatrix * vec4(position, 1.0);
  vWorld = world.xyz;
  vLocal = position;
  gl_Position = projectionMatrix * viewMatrix * world;
}`

const RING_FRAGMENT = `
uniform vec3 uColor;
uniform vec3 uSun;
uniform float uPower;
uniform float uInner;
uniform float uOuter;
uniform vec3 uPlanet;  // the planet's centre, in the world
uniform float uRadius; // and its radius
varying vec3 vWorld;
varying vec3 vLocal;
void main() {
  float t = (length(vLocal.xy) - uInner) / (uOuter - uInner);
  float bands = 0.55 + 0.25 * sin(t * 47.0 + sin(t * 13.0) * 2.0) + 0.2 * sin(t * 131.0);
  float gap = smoothstep(0.015, 0.045, abs(t - 0.63));
  float fade = smoothstep(0.0, 0.06, t) * (1.0 - smoothstep(0.9, 1.0, t));
  float a = clamp(bands, 0.0, 1.0) * gap * fade * 0.85;
  // In the planet's shadow: the way to the star passes through it.
  vec3 toSun = normalize(uSun - vWorld);
  vec3 oc = uPlanet - vWorld;
  float along = dot(oc, toSun);
  float shadow = along > 0.0 && length(oc - toSun * along) < uRadius ? 0.12 : 1.0;
  vec3 c = uColor * (0.08 + 0.92 * uPower * shadow);
  gl_FragColor = vec4(c, a);
}`

// Rings round a planet of radius r, from inner to outer (in its radii).
export function createRings(r, inner, outer, color) {
  return new THREE.Mesh(
    new THREE.RingGeometry(r * inner, r * outer, 160, 1),
    new THREE.ShaderMaterial({
      vertexShader: RING_VERTEX,
      fragmentShader: RING_FRAGMENT,
      uniforms: {
        uColor: { value: new THREE.Color(color) },
        uSun: { value: new THREE.Vector3() },
        uPower: { value: 1 },
        uInner: { value: r * inner },
        uOuter: { value: r * outer },
        uPlanet: { value: new THREE.Vector3() },
        uRadius: { value: r },
      },
      transparent: true,
      depthWrite: false,
      side: THREE.DoubleSide,
    }),
  )
}
