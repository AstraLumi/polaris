// The chapter as plain text for the clipboard. A dialogue's speaker is an
// attr, not text, so ProseMirror's own plain text (and a phone keyboard's
// "select all, copy") would lose it: here a dialogue reads "Name, manner:
// line", as it does when turned back into a paragraph. Notes, pictures and
// page breaks are left out; a scene break keeps its glyph. Tested by
// tests/editorScript.test.mjs.
import { speakerLead } from './nameMatch.js'

function inlineText(block) {
  let out = ''
  block.forEach((c) => {
    if (c.isText) out += c.text
    else if (c.type.name === 'hard_break') out += '\n'
    else if (c.type.name === 'tab') out += '\t'
  })
  return out
}

// firstLead false: the first block is a cut-off line, so no speaker for it.
function fragmentText(fragment, firstLead) {
  const blocks = []
  const walk = (parent) =>
    parent.forEach((n) => {
      if (n.isTextblock) {
        const lead = n.type.name === 'dialogue' && (firstLead || blocks.length) ? speakerLead(n.attrs) : ''
        const text = (lead + inlineText(n)).trimEnd()
        if (text) blocks.push(text)
      } else if (n.type.name === 'scene_break') {
        blocks.push(n.attrs.glyph || '* * *')
      } else if (n.type.name !== 'footnote') {
        walk(n)
      }
    })
  walk(fragment)
  return blocks.join('\n\n')
}

export const docText = (doc) => fragmentText(doc.content, true)

// For the editor's clipboardTextSerializer: a selection that starts part of
// the way through a line, or ends inside the line it started in, is only
// those words; one that holds the whole first line gets its speaker.
export function selectionText(slice, { $from, $to }) {
  const whole = slice.openStart === 0 || ($from.parentOffset === 0 && $to.pos >= $from.end())
  return fragmentText(slice.content, whole)
}

// One drawn page, from..to as pagination.js cuts them. A page may start or
// end inside a paragraph. A speaker is drawn above a line's start, so it goes
// with the page holding that start. A page's end is often the first
// character of the next page's block: that block is not on this page.
export function pageText(doc, from, to) {
  const $to = doc.resolve(to)
  let end = to
  for (let d = $to.depth; d > 0 && end === $to.start(d); d--) end = $to.before(d)
  const slice = doc.slice(from, Math.max(from, end), true)
  return fragmentText(slice.content, slice.openStart === 0 || doc.resolve(from).parentOffset === 0)
}

// Puts text on the clipboard; false if the browser refused. The async
// clipboard only exists on https or localhost, and Polaris may be served
// over plain http at home, so the old way is the fallback.
export async function copyToClipboard(text) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch (e) {
    return copyWithTextarea(text)
  }
}

function copyWithTextarea(text) {
  const back = document.activeElement
  const box = document.createElement('textarea')
  box.value = text
  box.readOnly = true // no on-screen keyboard for it
  box.style.cssText = 'position:fixed;left:-9999px;top:0;font-size:16px'
  document.body.appendChild(box)
  box.select()
  box.setSelectionRange(0, text.length)
  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch (e) {
    ok = false
  }
  box.remove()
  back?.focus?.({ preventScroll: true })
  return ok
}
