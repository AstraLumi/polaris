// Finds the stars in the sky picture (starCatalog.js) off the page's thread:
// reading an 8192 x 4096 picture and searching it takes a second or two.
// useStarCatalog.js talks to it.
import { findStars } from './starCatalog.js'

self.onmessage = async (e) => {
  const { id, url, options } = e.data
  try {
    const response = await fetch(url)
    if (!response.ok) throw new Error('the sky picture could not be loaded (' + response.status + ')')
    const bitmap = await createImageBitmap(await response.blob(), { colorSpaceConversion: 'none', premultiplyAlpha: 'none' })
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height)
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    ctx.drawImage(bitmap, 0, 0)
    bitmap.close()
    const image = ctx.getImageData(0, 0, canvas.width, canvas.height)
    const { stars, glow } = findStars(image, options)

    const n = stars.length
    const dirs = new Float32Array(n * 3)
    const mags = new Float32Array(n)
    const colors = new Uint8Array(n * 3)
    stars.forEach((s, i) => {
      dirs.set(s.dir, i * 3)
      mags[i] = s.mag
      for (let k = 0; k < 3; k++) colors[i * 3 + k] = parseInt(s.color.slice(1 + 2 * k, 3 + 2 * k), 16)
    })
    self.postMessage(
      { id, stars: { dirs, mags, colors }, glow: { width: glow.width, height: glow.height, data: glow.data } },
      [dirs.buffer, mags.buffer, colors.buffer, glow.data.buffer]
    )
  } catch (err) {
    self.postMessage({ id, error: String(err?.message || err) })
  }
}
