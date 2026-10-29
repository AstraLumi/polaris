// Measures a font's natural line height (browser only; the pure side is in
// css.js): a hidden span at 100px with line-height: normal, once the family
// has loaded. Cached per family. Word's "single" spacing is the font's own
// line spacing, so a paragraph's line-height multiple is times this ratio.
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
            return setLineRatio(family, measure(family)) || unknown
          })(),
        )
        if (await done.get(key)) changed = true
      } else await done.get(key)
    }),
  )
  return changed
}
