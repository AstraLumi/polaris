// The sky: a sphere round the camera, drawn behind everything. Each pixel
// looks up the sky picture by its direction, exactly as the format says
// (backend/cmd/skygen): the top edge is above the north pole (+z), the
// middle column above longitude 0 (+x), and east is to the left, as the sky
// looks from the ground. Shared by the map's globe and the solar system;
// keep the camera at its centre (mesh.position.copy(camera.position)).
import * as THREE from 'three'

const SKY_VERTEX = `
varying vec3 vDir;
void main() {
  vDir = position;
  vec4 p = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  gl_Position = p.xyww;
}`

const SKY_FRAGMENT = `
uniform sampler2D uSky;
uniform vec3 uLensDir; // towards a black hole, from the camera
uniform float uLensE;  // its Einstein angle (radians); 0 for none
varying vec3 vDir;
void main() {
  vec3 d = normalize(vDir);
  // Light passing a mass bends towards it: what is seen at angle th from
  // it comes from beta = th - E^2 / th (beyond it for small th). Near the
  // middle the stars smear into rings round it.
  if (uLensE > 0.0) {
    float th = acos(clamp(dot(d, uLensDir), -1.0, 1.0));
    if (th < uLensE * 6.0 && th > 1e-4) {
      float beta = th - uLensE * uLensE / th;
      vec3 side = normalize(d - uLensDir * cos(th));
      d = normalize(uLensDir * cos(beta) + side * sin(beta));
    }
  }
  float lon = atan(d.y, d.x);
  float lat = asin(clamp(d.z, -1.0, 1.0));
  vec2 uv = vec2(0.5 - lon / 6.283185307, 0.5 + lat / 3.141592654);
  gl_FragColor = vec4(texture2D(uSky, uv).rgb, 1.0);
}`

// Loads the picture at url and calls done(mesh) with the dome, unless
// cancel() was called first. Disposing the mesh's material also frees the
// picture (disposeSkyDome).
export function loadSkyDome(url, done) {
  let cancelled = false
  new THREE.TextureLoader().load(url, (tex) => {
    if (cancelled) return tex.dispose()
    // Sampled as it is, with no mipmaps: they would draw a seam where the
    // picture's edges meet.
    tex.colorSpace = THREE.NoColorSpace
    tex.generateMipmaps = false
    tex.minFilter = THREE.LinearFilter
    tex.wrapS = THREE.RepeatWrapping
    const mesh = new THREE.Mesh(
      new THREE.SphereGeometry(1, 48, 24),
      new THREE.ShaderMaterial({
        vertexShader: SKY_VERTEX,
        fragmentShader: SKY_FRAGMENT,
        uniforms: { uSky: { value: tex }, uLensDir: { value: new THREE.Vector3(1, 0, 0) }, uLensE: { value: 0 } },
        side: THREE.BackSide,
        depthWrite: false,
        depthTest: false,
      }),
    )
    mesh.renderOrder = -1
    mesh.frustumCulled = false
    done(mesh)
  })
  return () => {
    cancelled = true
  }
}

export function disposeSkyDome(mesh) {
  if (!mesh) return
  mesh.material.uniforms.uSky.value.dispose()
  mesh.material.dispose()
  mesh.geometry.dispose()
}
