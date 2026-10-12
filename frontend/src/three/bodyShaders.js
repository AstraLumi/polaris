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
// glow left on the night side.
const PLANET_VERTEX = `
varying vec3 vNormal;
varying vec3 vWorld;
void main() {
  vec4 world = modelMatrix * vec4(position, 1.0);
  vWorld = world.xyz;
  vNormal = normalize(mat3(modelMatrix) * normal);
  gl_Position = projectionMatrix * viewMatrix * world;
}`

const PLANET_FRAGMENT = `
uniform vec3 uColor;
uniform vec3 uSun;
varying vec3 vNormal;
varying vec3 vWorld;
void main() {
  float d = dot(normalize(vNormal), normalize(uSun - vWorld));
  float day = smoothstep(-0.12, 0.35, d);
  vec3 night = uColor * 0.05 + vec3(0.01, 0.015, 0.03);
  gl_FragColor = vec4(mix(night, uColor * (0.25 + 0.85 * max(d, 0.0)), day), 1.0);
}`

export function createPlanetMaterial(color) {
  return new THREE.ShaderMaterial({
    vertexShader: PLANET_VERTEX,
    fragmentShader: PLANET_FRAGMENT,
    uniforms: { uColor: { value: new THREE.Color(color) }, uSun: { value: new THREE.Vector3() } },
  })
}
