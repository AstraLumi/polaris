// Shadows cast by the star's light, for the solar system's shaders: the star
// is a disc of light (uLightRadius, in the scene), every occluder a sphere
// (uOccluders: centre and radius, up to MAX_CASTERS, picked by eclipses.js),
// and sunlight(p, sun) is how much of the disc is seen from p, 0 to 1. Full
// shadow (umbra), soft edges (penumbra) and annular eclipses all come out of
// the overlap of two discs. With no occluders (uOccluderCount 0, as on the
// map's own globe) it is always 1. uLightColor is the colour of the star's
// light (white on the map). An occluder with air (its radius given negative)
// bends a little red light into its shadow: sunlight() leaves how much of
// the disc such bodies hide in airGlow, for a blood-red eclipsed moon.
//
// Light from all round (an enclosing shell that glows, lighting.js) comes
// on top of the star's: uFill is how strong it is (1 about a clear day's
// light), uFillColor its colour. No direction, so no shadows and no night
// side; 0 when nothing encloses the system.
//
// Rings shadow their own planet too: ringShade(p, sun) is how much light
// gets through the rings (uRingOuter 0 for a planet without any), with the
// same bands ringAlpha draws them with.
import * as THREE from 'three'
import { MAX_CASTERS } from '../eclipses'

export const SHADOW_GLSL = `
uniform vec4 uOccluders[${MAX_CASTERS}];
uniform int uOccluderCount;
uniform float uLightRadius;
uniform vec3 uLightColor;
uniform float uFill;
uniform vec3 uFillColor;
uniform vec3 uRingCenter;
uniform vec3 uRingNormal;
uniform float uRingInner;
uniform float uRingOuter;
// How opaque the rings are at t, 0 at the inner edge to 1 at the outer.
float ringAlpha(float t) {
  float bands = 0.55 + 0.25 * sin(t * 47.0 + sin(t * 13.0) * 2.0) + 0.2 * sin(t * 131.0);
  float gap = smoothstep(0.015, 0.045, abs(t - 0.63));
  float fade = smoothstep(0.0, 0.06, t) * (1.0 - smoothstep(0.9, 1.0, t));
  return clamp(bands, 0.0, 1.0) * gap * fade * 0.85;
}
float ringShade(vec3 p, vec3 sun) {
  if (uRingOuter <= 0.0) return 1.0;
  vec3 dir = normalize(sun - p);
  float dn = dot(dir, uRingNormal);
  if (abs(dn) < 1e-4) return 1.0;
  float t = dot(uRingCenter - p, uRingNormal) / dn;
  if (t <= 0.0) return 1.0;
  float x = (length(p + dir * t - uRingCenter) - uRingInner) / (uRingOuter - uRingInner);
  if (x <= 0.0 || x >= 1.0) return 1.0;
  return 1.0 - ringAlpha(x);
}
// The area two discs of angular radii a and b, c apart, share.
float discOverlap(float a, float b, float c) {
  if (c >= a + b) return 0.0;
  if (c <= abs(a - b)) {
    float m = min(a, b);
    return 3.14159265 * m * m;
  }
  float a2 = a * a;
  float b2 = b * b;
  float x = clamp((c * c + a2 - b2) / (2.0 * c * a), -1.0, 1.0);
  float y = clamp((c * c + b2 - a2) / (2.0 * c * b), -1.0, 1.0);
  return a2 * acos(x) + b2 * acos(y) - 0.5 * sqrt(max((-c + a + b) * (c + a - b) * (c - a + b) * (c + a + b), 0.0));
}
float airGlow = 0.0;
// The red light of an eclipse, for what lies in the shadow of a body with air.
const vec3 EARTHSHADOW = vec3(0.42, 0.1, 0.035);
float sunlight(vec3 p, vec3 sun) {
  airGlow = 0.0;
  if (uOccluderCount == 0) return 1.0;
  vec3 toSun = sun - p;
  float ds = length(toSun);
  vec3 dir = toSun / ds;
  float a = asin(min(uLightRadius / ds, 1.0));
  float hidden = 0.0;
  for (int k = 0; k < ${MAX_CASTERS}; k++) {
    if (k >= uOccluderCount) break;
    vec3 to = uOccluders[k].xyz - p;
    float d = length(to);
    if (d >= ds || dot(to, dir) <= 0.0) continue;
    float b = asin(min(abs(uOccluders[k].w) / d, 1.0));
    float c = acos(clamp(dot(to / d, dir), -1.0, 1.0));
    float h = discOverlap(a, b, c);
    hidden += h;
    if (uOccluders[k].w < 0.0) airGlow += h;
  }
  airGlow = clamp(airGlow / (3.14159265 * a * a), 0.0, 1.0);
  return clamp(1.0 - hidden / (3.14159265 * a * a), 0.0, 1.0);
}`

export const shadowUniforms = () => ({
  uOccluders: { value: Array.from({ length: MAX_CASTERS }, () => new THREE.Vector4()) },
  uOccluderCount: { value: 0 },
  uLightRadius: { value: 1 },
  uLightColor: { value: new THREE.Color(1, 1, 1) },
  uFill: { value: 0 },
  uFillColor: { value: new THREE.Color(1, 1, 1) },
  uRingCenter: { value: new THREE.Vector3() },
  uRingNormal: { value: new THREE.Vector3(0, 0, 1) },
  uRingInner: { value: 0 },
  uRingOuter: { value: 0 },
})

// Points a material's shadow uniforms at its occluders: [x, y, z, r] each,
// and gives it the star's light (radius, colour).
export function setOccluders(uniforms, occluders, lightRadius, lightColor) {
  const n = Math.min(occluders.length, MAX_CASTERS)
  for (let k = 0; k < n; k++) uniforms.uOccluders.value[k].fromArray(occluders[k])
  uniforms.uOccluderCount.value = n
  uniforms.uLightRadius.value = lightRadius
  if (lightColor) uniforms.uLightColor.value.copy(lightColor)
}

// The light from all round a material gets: its strength and colour.
export function setFill(uniforms, fill, color) {
  uniforms.uFill.value = fill
  if (color) uniforms.uFillColor.value.copy(color)
}

// The rings that shadow a planet: their centre and the normal of their
// plane (world), their edges; null for none.
export function setRingShadow(uniforms, ring) {
  uniforms.uRingOuter.value = ring ? ring.outer : 0
  if (!ring) return
  uniforms.uRingCenter.value.copy(ring.center)
  uniforms.uRingNormal.value.copy(ring.normal)
  uniforms.uRingInner.value = ring.inner
}
