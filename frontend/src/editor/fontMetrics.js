// A font's line spacing ratio (browser only; the pure side is in css.js).
// Word's "single" spacing on Windows is the font's Windows line metrics
// (usWinAscent + usWinDescent + external leading), which the server reads
// from the font file (GET /api/fonts/line-metrics, `win`). Without it (a
// font the server can't find, offline, no server) the ratio kept from last
// time, else the browser's own `line-height: normal`, measured on a hidden
// span at 100px once the family has loaded. Cached per family, and kept in
// localStorage so a book opened offline draws the same lines.
import { setLineRatio, lineRatio, familyCSS } from './css.js'

const done = new Map() // lower-case family → promise

function measure(family) {
  const span = document.createElement('span')
  span.textContent = 'Hxgy'
  span.style.cssText = `position:absolute;visibility:hidden;left:-9999px;top:0;white-space:nowrap;font-size:100px;line-height:normal;font-weight:400;font-style:normal;font-family:${familyCSS(family)}`
  document.body.appendChild(span)
  const h = span.getBoundingClientRect().height
  span.remove()
  return h / 100
}

const KEPT = 'polaris-line-ratios'
const kept = () => {
  try {
    return JSON.parse(localStorage.getItem(KEPT) || '{}') || {}
  } catch (e) {
    return {}
  }
}
const keep = (key, r) => {
  try {
    localStorage.setItem(KEPT, JSON.stringify({ ...kept(), [key]: r }))
  } catch (e) {
    /* not kept */
  }
}

// The server's Windows ratio for a family, or null.
async function serverRatio(family) {
  try {
    const res = await fetch(`/api/fonts/line-metrics?family=${encodeURIComponent(family)}`)
    if (!res.ok) return null
    const win = +(await res.json())?.win
    return win > 0 ? win : null
  } catch (e) {
    return null
  }
}

// Measures each family (loading it first); true when a ratio changed.
export async function measureRatios(families) {
  if (typeof document === 'undefined') return false
  let changed = false
  await Promise.all(
    [...new Set((families || []).filter(Boolean))].map(async (family) => {
      const key = family.toLowerCase()
      if (!done.has(key)) {
        done.set(
          key,
          (async () => {
            try {
              await document.fonts?.load(`16px ${familyCSS(family)}`)
              await document.fonts?.ready
            } catch (e) {
              /* measured with whatever it is drawn in */
            }
            const unknown = lineRatio(family) == null
            const ratio = (await serverRatio(family)) ?? kept()[key] ?? measure(family)
            if (ratio !== kept()[key]) keep(key, ratio)
            return setLineRatio(family, ratio) || unknown
          })(),
        )
        if (await done.get(key)) changed = true
      } else await done.get(key)
    }),
  )
  return changed
}
