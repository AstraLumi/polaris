// How the solar system's bodies look (SolarScene.vue): a star with a soft,
// shimmering corona and a slightly darker rim, a black hole as a dark disc
// in a bright ring, and planets lit by their star with a soft line between
// day and night. Only lazily loaded pages import this (three.js).
import * as THREE from 'three'

// ---- Stars ------------------------------------------------------------------------------

// The star's face: brightest in the middle, a little darker and redder at
// the rim (limb darkening), as real stars look.
const STAR_VERTEX = `
varying vec3 vNormal;
varying vec3 vView;
void main() {
  vec4 world = modelMatrix * vec4(position, 1.0);
  vNormal = normalize(mat3(modelMatrix) * normal);
  vView = normalize(cameraPosition - world.xyz);
  gl_Position = projectionMatrix * viewMatrix * world;
}`

const STAR_FRAGMENT = `
uniform vec3 uColor;
uniform float uDark;
varying vec3 vNormal;
varying vec3 vView;
void main() {
  float mu = clamp(dot(normalize(vNormal), normalize(vView)), 0.0, 1.0);
  vec3 face = mix(uColor * vec3(1.0, 0.82, 0.7) * 0.75, mix(uColor, vec3(1.0), 0.35), pow(mu, 0.45));
  gl_FragColor = vec4(mix(face, vec3(0.0), uDark), 1.0);
}`

// The corona: a flat glow always facing the camera, fading out from the
// star with slow rays turning round it. For a black hole, a bright ring
// just outside the dark disc instead.
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
uniform float uRing;   // 1 for a black hole
varying vec2 vUv;
void main() {
  float d = length(vUv);
  if (d > 1.0) discard;
  float a = atan(vUv.y, vUv.x);
  float x = max(d - uCore, 0.0) / (1.0 - uCore);
  float rays = 0.75 + 0.25 * sin(a * 9.0 + uTime * 0.35) * sin(a * 5.0 - uTime * 0.21);
  float glow = pow(1.0 - x, 3.0) * rays;
  float ring = exp(-pow((d - uCore * 1.25) / (uCore * 0.18), 2.0));
  float k = mix(glow * 0.85, ring * 1.2 + glow * 0.35, uRing);
  if (d < uCore && uRing < 0.5) k = 0.0;
  gl_FragColor = vec4(uColor * k, k);
}`

// A star of radius r: { group, corona, update(time) }. The corona must face
// the camera (faceCamera) before each render.
export function createStar(r, color, blackHole = false) {
  const group = new THREE.Group()
  const c = new THREE.Color(color)
  const face = new THREE.Mesh(
    new THREE.SphereGeometry(r, 48, 24),
    new THREE.ShaderMaterial({
      vertexShader: STAR_VERTEX,
      fragmentShader: STAR_FRAGMENT,
      uniforms: { uColor: { value: c }, uDark: { value: blackHole ? 1 : 0 } },
    }),
  )
  const size = r * (blackHole ? 3.2 : 4.5)
  const corona = new THREE.Mesh(
    new THREE.PlaneGeometry(size * 2, size * 2),
    new THREE.ShaderMaterial({
      vertexShader: CORONA_VERTEX,
      fragmentShader: CORONA_FRAGMENT,
      uniforms: {
        uColor: { value: blackHole ? new THREE.Color('#ff9a4a') : c },
        uTime: { value: 0 },
        uCore: { value: r / size },
        uRing: { value: blackHole ? 1 : 0 },
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
  float day = smoothstep(-0.12, 0.35, d);
  vec3 night = base * 0.05 + vec3(0.01, 0.015, 0.03);
  gl_FragColor = vec4(mix(night, base * (0.25 + 0.85 * max(d, 0.0)), day), 1.0);
}`

export function createPlanetMaterial(color, { bands = false } = {}) {
  return new THREE.ShaderMaterial({
    vertexShader: PLANET_VERTEX,
    fragmentShader: PLANET_FRAGMENT,
    uniforms: { uColor: { value: new THREE.Color(color) }, uSun: { value: new THREE.Vector3() }, uBands: { value: bands ? 1 : 0 } },
  })
}

// ---- Atmospheres ----------------------------------------------------------------------
// A thin glow round the planet's edge (a shell a little larger than it,
// bright where it is seen edge-on), stronger on the side the star lights.
const ATMOSPHERE_FRAGMENT = `
uniform vec3 uColor;
uniform vec3 uSun;
varying vec3 vNormal;
varying vec3 vWorld;
void main() {
  vec3 n = normalize(vNormal);
  float rim = pow(1.0 - max(dot(n, normalize(cameraPosition - vWorld)), 0.0), 3.0);
  float lit = smoothstep(-0.3, 0.45, dot(n, normalize(uSun - vWorld)));
  gl_FragColor = vec4(uColor * rim * (0.12 + 1.1 * lit), 1.0);
}`

export function createAtmosphere(r, color) {
  return new THREE.Mesh(
    new THREE.SphereGeometry(r * 1.06, 48, 24),
    new THREE.ShaderMaterial({
      vertexShader: PLANET_VERTEX,
      fragmentShader: ATMOSPHERE_FRAGMENT,
      uniforms: { uColor: { value: new THREE.Color(color) }, uSun: { value: new THREE.Vector3() } },
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
  float day = smoothstep(-0.15, 0.3, d);
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
      uniforms: { uSun: { value: new THREE.Vector3() }, uTime: { value: 0 } },
      transparent: true,
      depthWrite: false,
    }),
  )
}
