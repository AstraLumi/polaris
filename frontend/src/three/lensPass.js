// Gravitational lensing round a black hole, for the solar system's view: the
// scene is drawn into a texture, then onto the screen with everything
// behind the black hole bent round it (a point-mass lens: what is seen at a
// distance th from it comes from th - E²/th, so close in, the far side
// shows too). What is in front of it isn't bent, nor its own black disc and
// photon ring. Only used while a black hole is in view.
import * as THREE from 'three'

const VERTEX = `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = vec4(position.xy, 0.0, 1.0);
}`

const FRAGMENT = `
uniform sampler2D uColor;
uniform sampler2D uDepth;
uniform vec2 uCenter;   // the black hole, in uv
uniform vec2 uRes;      // the screen, in pixels
uniform float uE;       // its Einstein radius, in pixels
uniform float uKeep;    // its shadow and ring, in pixels: not bent
uniform float uHoleDepth;
varying vec2 vUv;
void main() {
  vec2 c = uCenter * uRes;
  vec2 off = vUv * uRes - c;
  float th = length(off);
  vec2 uv = vUv;
  // Behind the black hole (farther than its centre) and outside its ring.
  if (th > uKeep && th < uE * 8.0 && texture2D(uDepth, vUv).x > uHoleDepth) {
    float fade = 1.0 - smoothstep(uE * 5.0, uE * 8.0, th);
    float beta = th - uE * uE / th * fade;
    uv = (c + off / th * beta) / uRes;
  }
  gl_FragColor = texture2D(uColor, uv);
}`

export function createLensPass() {
  const target = new THREE.WebGLRenderTarget(1, 1, { samples: 4, depthTexture: new THREE.DepthTexture(1, 1) })
  const material = new THREE.ShaderMaterial({
    vertexShader: VERTEX,
    fragmentShader: FRAGMENT,
    uniforms: {
      uColor: { value: target.texture },
      uDepth: { value: target.depthTexture },
      uCenter: { value: new THREE.Vector2() },
      uRes: { value: new THREE.Vector2(1, 1) },
      uE: { value: 0 },
      uKeep: { value: 0 },
      uHoleDepth: { value: 1 },
    },
    depthTest: false,
    depthWrite: false,
  })
  const quad = new THREE.Mesh(new THREE.PlaneGeometry(2, 2), material)
  quad.frustumCulled = false
  const screen = new THREE.Scene()
  screen.add(quad)
  const flat = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1)
  const v = new THREE.Vector3()

  return {
    setSize(width, height) {
      target.setSize(width, height)
      material.uniforms.uRes.value.set(width, height)
    },
    // Draws the scene with the lens of a black hole at `at` (world), of
    // drawn radius r. False when it isn't in view (nothing drawn).
    render(renderer, scene, camera, at, r) {
      v.copy(at).project(camera)
      if (v.z < -1 || v.z > 1 || Math.abs(v.x) > 1.6 || Math.abs(v.y) > 1.6) return false
      const dist = camera.position.distanceTo(at)
      const pxPerTan = target.height / 2 / Math.tan((camera.fov * Math.PI) / 360)
      const u = material.uniforms
      u.uCenter.value.set(v.x * 0.5 + 0.5, v.y * 0.5 + 0.5)
      u.uE.value = (pxPerTan * (r * 3)) / dist
      u.uKeep.value = (pxPerTan * (r * 1.3)) / dist
      u.uHoleDepth.value = v.z * 0.5 + 0.5
      renderer.setRenderTarget(target)
      renderer.render(scene, camera)
      renderer.setRenderTarget(null)
      renderer.render(screen, flat)
      return true
    },
    dispose() {
      target.depthTexture?.dispose()
      target.dispose()
      material.dispose()
      quad.geometry.dispose()
    },
  }
}
