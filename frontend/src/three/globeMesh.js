// The planet as a three.js mesh: every cell of the grid (planetGrid.js) as a
// fan of triangles, coloured from a texture with one pixel per cell, so a
// repaint only rewrites pixels. Shared by the map's globe (PlanetGlobe.vue)
// and the solar system (SolarScene.vue). Only imported by pages that load
// three.js lazily, so three stays out of the main bundle.
import * as THREE from 'three'
import { SHADOW_GLSL, shadowUniforms } from './shadows'

export const SEA = [27, 45, 74]
const COLUMNS = 2048 // the colour texture's width

const hexRgb = (hex) => {
  const n = parseInt(hex.slice(1), 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

const VERTEX = `
attribute float aCell;
attribute float aEdge;
uniform sampler2D uColors;
uniform vec2 uSize;
varying vec3 vColor;
varying float vEdge;
varying vec3 vNormal;
varying vec3 vWorld;
void main() {
  vWorld = (modelMatrix * vec4(position, 1.0)).xyz;
  float x = mod(aCell, uSize.x);
  float y = floor(aCell / uSize.x);
  vColor = texture2D(uColors, vec2((x + 0.5) / uSize.x, (y + 0.5) / uSize.y)).rgb;
  vEdge = aEdge;
  vNormal = normalize(position);
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`

// uLight is the direction of the light in the mesh's own frame; uAmbient how
// lit the far side still is (the map keeps it bright, space doesn't). In the
// solar system, uSun (the star, in the world) and the occluders cast other
// bodies' shadows on it (three/shadows.js); the map's globe has none.
const FRAGMENT = `
uniform vec3 uLight;
uniform vec3 uSun;
uniform float uGrid;
uniform float uAmbient;
uniform float uTwilight;
uniform float uPower;
varying vec3 vColor;
varying float vEdge;
varying vec3 vNormal;
varying vec3 vWorld;
${SHADOW_GLSL}
void main() {
  float d = dot(vNormal, uLight);
  float lit = uTwilight > 0.0 ? smoothstep(-uTwilight, uTwilight * 3.0, d) * (0.3 + 0.7 * max(d, 0.0)) : max(d, 0.0);
  lit *= sunlight(vWorld, uSun) * ringShade(vWorld, uSun);
  vec3 c = vColor * (uAmbient + (1.0 - uAmbient) * lit * uPower * uLightColor);
  float line = smoothstep(0.84, 0.96, vEdge) * uGrid;
  gl_FragColor = vec4(mix(c, c * 0.5, line), 1.0);
}`

// One fan of triangles per cell: its centre and its corners, each vertex
// carrying the cell's number (for its colour) and whether it is a corner
// (for the outline). colorOf(cell) is '#rrggbb', or '' for open sea.
// twilight: how wide the soft band between day and night is (0 for a hard
// edge, as on the map's own globe).
export function createGlobeMesh(grid, colorOf, { ambient = 0.6, twilight = 0 } = {}) {
  const n = grid.count
  const pos = new Float32Array(n * 7 * 3)
  const cellAttr = new Float32Array(n * 7)
  const edge = new Float32Array(n * 7)
  const index = new Uint32Array(n * 6 * 3)
  let v = 0
  let ix = 0
  for (let c = 0; c < n; c++) {
    const corners = grid.corners(c)
    const base = v
    pos.set(grid.center(c), v * 3)
    cellAttr[v] = c
    edge[v] = 0
    v++
    for (const k of corners) {
      pos.set(k, v * 3)
      cellAttr[v] = c
      edge[v] = 1
      v++
    }
    const m = corners.length
    for (let k = 0; k < m; k++) {
      index[ix++] = base
      index[ix++] = base + 1 + k
      index[ix++] = base + 1 + ((k + 1) % m)
    }
  }
  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(pos.subarray(0, v * 3), 3))
  geo.setAttribute('aCell', new THREE.BufferAttribute(cellAttr.subarray(0, v), 1))
  geo.setAttribute('aEdge', new THREE.BufferAttribute(edge.subarray(0, v), 1))
  geo.setIndex(new THREE.BufferAttribute(index.subarray(0, ix), 1))

  const rows = Math.ceil(n / COLUMNS)
  const colorData = new Uint8Array(COLUMNS * rows * 4)
  const colorTex = new THREE.DataTexture(colorData, COLUMNS, rows, THREE.RGBAFormat, THREE.UnsignedByteType)
  colorTex.magFilter = THREE.NearestFilter
  colorTex.minFilter = THREE.NearestFilter
  const material = new THREE.ShaderMaterial({
    vertexShader: VERTEX,
    fragmentShader: FRAGMENT,
    uniforms: {
      uColors: { value: colorTex },
      uSize: { value: new THREE.Vector2(COLUMNS, rows) },
      uLight: { value: new THREE.Vector3(0, 0, 1) },
      uGrid: { value: 0 },
      uAmbient: { value: ambient },
      uTwilight: { value: twilight },
      uPower: { value: 1 },
      uSun: { value: new THREE.Vector3() },
      ...shadowUniforms(),
    },
  })
  const mesh = new THREE.Mesh(geo, material)

  const writeColor = (cell) => {
    const col = colorOf(cell)
    const [r, g, b] = col ? hexRgb(col) : SEA
    colorData.set([r, g, b, 255], cell * 4)
  }
  const globe = {
    mesh,
    material,
    recolor(cells) {
      for (const c of cells) writeColor(c)
      colorTex.needsUpdate = true
    },
    recolorAll() {
      for (let c = 0; c < n; c++) writeColor(c)
      colorTex.needsUpdate = true
    },
    dispose() {
      geo.dispose()
      material.dispose()
      colorTex.dispose()
    },
  }
  globe.recolorAll()
  return globe
}
