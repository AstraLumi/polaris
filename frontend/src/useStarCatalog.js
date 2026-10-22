// The stars of the story's sky picture, found off the page's thread
// (starCatalog.worker.js) and kept: opening the sky view again, or another
// view of the same picture, does not search it twice.
import { CATALOGUE_VERSION } from './starCatalog.js'

const cache = new Map() // url + options + version -> the promise of the catalogue

// url: the sky picture. options: those of findStars (starCatalog.js).
// Answers a promise of { stars: [{ dir: [x, y, z], mag, color: '#rrggbb' }],
// brightest first, glow: { width, height, data } } (the glow as RGBA bytes,
// half the picture's size, the stars taken out).
export function loadStarCatalog(url, options = {}) {
  const key = CATALOGUE_VERSION + '|' + url + '|' + JSON.stringify(options, Object.keys(options).sort())
  if (cache.has(key)) return cache.get(key)
  const promise = new Promise((resolve, reject) => {
    const worker = new Worker(new URL('./starCatalog.worker.js', import.meta.url), { type: 'module' })
    worker.onmessage = (e) => {
      worker.terminate()
      if (e.data.error) return reject(new Error(e.data.error))
      const { dirs, mags, colors } = e.data.stars
      const hex = (v) => v.toString(16).padStart(2, '0')
      const stars = []
      for (let i = 0; i < mags.length; i++) {
        stars.push({
          dir: [dirs[i * 3], dirs[i * 3 + 1], dirs[i * 3 + 2]],
          mag: mags[i],
          color: '#' + hex(colors[i * 3]) + hex(colors[i * 3 + 1]) + hex(colors[i * 3 + 2]),
        })
      }
      resolve({ stars, glow: e.data.glow })
    }
    worker.onerror = (e) => {
      worker.terminate()
      reject(new Error(e.message || 'the stars could not be found'))
    }
    worker.postMessage({ id: 1, url, options })
  })
  // A failure is not remembered: asking again tries again.
  promise.catch(() => cache.delete(key))
  cache.set(key, promise)
  return promise
}
