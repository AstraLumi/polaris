// The writing editor's document: exactly the nodes, marks and attrs of
// docs/EDITOR.md, with how each is drawn and how it is read back from
// HTML (our own copies, and pastes from Word, Google Docs and web pages:
// the formatting the schema knows is kept, the rest dropped).
//
// Stored documents are ProseMirror JSON. Attributes this schema doesn't know
// (written by a newer Polaris, or the .docx import) are kept: docFromJSON
// tucks them into an `extra` attr and docToJSON puts them back.
import { Schema } from 'prosemirror-model'
import {
  cssColorToHex,
  cssFontFamily,
  cssFontSizeToPt,
  familyCSS,
  num,
  paraDeclarations,
  paraFromCSS,
} from './css.js'

// Paragraph formatting, shared by paragraphs and headings.
export const PARA_ATTRS = ['style', 'align', 'indentFirst', 'indentLeft', 'indentRight', 'spaceBefore', 'spaceAfter', 'lineHeight', 'keepNext']
const ALIGN_VALUES = new Set(['left', 'center', 'right', 'justify'])

const paraAttrSpec = () => ({
  style: { default: null },
  align: { default: null },
  indentFirst: { default: null },
  indentLeft: { default: null },
  indentRight: { default: null },
  spaceBefore: { default: null },
  spaceAfter: { default: null },
  lineHeight: { default: null },
  keepNext: { default: false },
  extra: { default: null },
})

// The paragraph's own formatting (everything but the style), nulls left out.
export function ownPara(attrs) {
  const out = {}
  for (const k of PARA_ATTRS) {
    if (k === 'style') continue
    if (k === 'keepNext' ? attrs[k] : attrs[k] != null) out[k] = attrs[k]
  }
  return out
}

function cleanPara(raw) {
  const out = {}
  if (!raw || typeof raw !== 'object') return out
  for (const k of PARA_ATTRS) {
    const v = raw[k]
    if (v === undefined || v === null) continue
    if (k === 'style') out.style = String(v)
    else if (k === 'align') {
      if (ALIGN_VALUES.has(v)) out.align = v
    } else if (k === 'keepNext') out.keepNext = !!v
    else if (Number.isFinite(+v)) out[k] = +v
  }
  if (raw.extra && typeof raw.extra === 'object') out.extra = raw.extra
  return out
}

function paraToDOM(tag, node) {
  const a = node.attrs
  const dom = {}
  if (a.style != null) dom['data-style'] = a.style
  const own = ownPara(a)
  if (Object.keys(own).length || a.extra) dom['data-para'] = JSON.stringify(a.extra ? { ...own, extra: a.extra } : own)
  const css = paraDeclarations(own).join('; ')
  if (css) dom.style = css
  return [tag, dom, 0]
}

// Our own copies carry the exact attrs in data-para; anything else is read
// from its inline style.
function paraFromDOM(el) {
  let attrs
  const data = el.getAttribute('data-para')
  if (data) {
    try {
      attrs = cleanPara(JSON.parse(data))
    } catch (e) {
      attrs = {}
    }
  } else attrs = cleanPara(paraFromCSS(el.style))
  const style = el.getAttribute('data-style')
  if (style) attrs.style = style
  return attrs
}

const keep = (spec) => ({ ...spec, extra: { default: null } })

// A dialogue as HTML: the speaker (small caps) and manner (italics) above
// the line, which is the editable part.
export function dialogueDOM(attrs) {
  const { character, speaker, manner } = attrs
  const dom = { class: 'pm-dialogue', 'data-dialogue': '', 'data-speaker': speaker || '', 'data-manner': manner || '' }
  if (character != null) dom['data-character'] = String(character)
  const who = ['span', { class: 'pm-speaker' }, speaker || '…']
  const who2 = manner ? [who, ['span', { class: 'pm-manner' }, `, ${manner}`]] : [who]
  return ['div', dom, ['div', { class: 'pm-who', contenteditable: 'false' }, ...who2], ['p', { class: 'pm-line' }, 0]]
}

const isUpload = (src) => typeof src === 'string' && src.startsWith('/uploads/')

export const schema = new Schema({
  nodes: {
    // mode: null (prose, the default) or 'script' (stage 4: Script Mode).
    doc: { content: 'block+', attrs: { mode: { default: null }, extra: { default: null } } },

    paragraph: {
      group: 'block',
      content: 'inline*',
      attrs: paraAttrSpec(),
      parseDOM: [{ tag: 'p', getAttrs: paraFromDOM }],
      toDOM: (node) => paraToDOM('p', node),
    },

    heading: {
      group: 'block',
      content: 'inline*',
      defining: true,
      attrs: { level: { default: 1 }, ...paraAttrSpec() },
      parseDOM: [1, 2, 3, 4, 5, 6].map((level) => ({ tag: `h${level}`, getAttrs: (el) => ({ ...paraFromDOM(el), level }) })),
      toDOM: (node) => paraToDOM(`h${Math.min(6, Math.max(1, node.attrs.level | 0 || 1))}`, node),
    },

    // A scene break: a glyph line ("* * *") or an uploaded ornament.
    scene_break: {
      group: 'block',
      atom: true,
      selectable: true,
      attrs: keep({ glyph: { default: '* * *' }, image: { default: null } }),
      parseDOM: [
        {
          tag: 'div[data-scene-break]',
          getAttrs: (el) => ({ glyph: el.getAttribute('data-glyph') || '* * *', image: el.getAttribute('data-image') || null }),
        },
        { tag: 'hr', getAttrs: () => ({}) },
      ],
      toDOM: (node) => {
        const { glyph, image } = node.attrs
        const dom = { class: 'scene-break', 'data-scene-break': '', 'data-glyph': glyph || '* * *' }
        if (image) {
          dom['data-image'] = image
          return ['div', dom, ['img', { src: image, alt: glyph || '' }]]
        }
        return ['div', dom, glyph || '* * *']
      },
    },

    // A forced new page (ignored for Online Post). Its label is drawn by CSS.
    page_break: {
      group: 'block',
      atom: true,
      selectable: true,
      attrs: keep({}),
      parseDOM: [
        { tag: 'div[data-page-break]' },
        // Word: <br clear=all style='page-break-before:always'>
        { tag: 'br', priority: 60, getAttrs: (el) => (/page-break-before\s*:\s*always|break-before\s*:\s*page/i.test(el.getAttribute('style') || '') ? {} : false) },
      ],
      toDOM: () => ['div', { class: 'page-break', 'data-page-break': '' }],
    },

    // A table of contents, generated from the book's headings: the editor
    // lists the chapters (a node view), the page view adds page numbers.
    toc: {
      group: 'block',
      atom: true,
      selectable: true,
      attrs: keep({ levels: { default: 1 }, title: { default: 'Contents' } }),
      parseDOM: [
        {
          tag: 'nav[data-toc]',
          getAttrs: (el) => ({ levels: Math.min(3, Math.max(1, +el.getAttribute('data-levels') || 1)), title: el.getAttribute('data-title') ?? 'Contents' }),
        },
      ],
      toDOM: (node) => [
        'nav',
        { class: 'pm-toc', 'data-toc': '', 'data-levels': String(node.attrs.levels || 1), 'data-title': node.attrs.title ?? '' },
        ['p', { class: 'pm-toc-title' }, node.attrs.title ?? ''],
      ],
    },

    // A line of dialogue (stage 4, Script Mode): who speaks (a character's id
    // or null, and the name as shown), how, and the spoken line as content.
    // The speaker part is drawn but not editable text (it is attrs).
    dialogue: {
      group: 'block',
      content: 'inline*',
      defining: true,
      attrs: keep({ character: { default: null }, speaker: { default: '' }, manner: { default: '' } }),
      parseDOM: [
        {
          tag: 'div[data-dialogue]',
          contentElement: '.pm-line',
          getAttrs: (el) => {
            const id = parseInt(el.getAttribute('data-character'), 10)
            return { character: id > 0 ? id : null, speaker: el.getAttribute('data-speaker') || '', manner: el.getAttribute('data-manner') || '' }
          },
        },
      ],
      toDOM: (node) => dialogueDOM(node.attrs),
    },

    image: {
      group: 'block',
      atom: true,
      draggable: true,
      attrs: keep({ src: { default: '' }, alt: { default: '' }, width: { default: null }, align: { default: 'center' }, fullPage: { default: false } }),
      parseDOM: [
        {
          tag: 'figure.pm-image',
          getAttrs: (el) => {
            const img = el.querySelector('img')
            const src = img?.getAttribute('src') || ''
            if (!isUpload(src)) return false
            const w = parseFloat(el.getAttribute('data-width'))
            return {
              src,
              alt: img.getAttribute('alt') || '',
              width: Number.isFinite(w) ? w : null,
              align: el.getAttribute('data-align') || 'center',
              fullPage: el.hasAttribute('data-full-page'),
            }
          },
        },
        // Pictures pasted from elsewhere aren't uploads yet; they are left out.
        { tag: 'img[src]', getAttrs: (el) => (isUpload(el.getAttribute('src')) ? { src: el.getAttribute('src'), alt: el.getAttribute('alt') || '' } : false) },
      ],
      toDOM: (node) => {
        const { src, alt, width, align, fullPage } = node.attrs
        const dom = { class: 'pm-image', 'data-align': align || 'center' }
        if (width != null) dom['data-width'] = String(width)
        if (fullPage) dom['data-full-page'] = ''
        return ['figure', dom, ['img', { src, alt, style: width != null ? `width: ${num(width)}pt` : null }]]
      },
    },

    bullet_list: {
      group: 'block',
      content: 'list_item+',
      attrs: keep({}),
      parseDOM: [{ tag: 'ul' }],
      toDOM: () => ['ul', 0],
    },

    ordered_list: {
      group: 'block',
      content: 'list_item+',
      attrs: keep({ start: { default: 1 } }),
      parseDOM: [{ tag: 'ol', getAttrs: (el) => ({ start: el.hasAttribute('start') ? +el.getAttribute('start') || 1 : 1 }) }],
      toDOM: (node) => ['ol', node.attrs.start === 1 ? {} : { start: node.attrs.start }, 0],
    },

    list_item: {
      // A paragraph, then more blocks: paragraphs and nested lists.
      content: 'paragraph block*',
      defining: true,
      attrs: keep({}),
      parseDOM: [{ tag: 'li' }],
      toDOM: () => ['li', 0],
    },

    text: { group: 'inline' },

    hard_break: {
      inline: true,
      group: 'inline',
      selectable: false,
      attrs: keep({}),
      parseDOM: [{ tag: 'br' }],
      toDOM: () => ['br'],
    },

    tab: {
      inline: true,
      group: 'inline',
      atom: true,
      selectable: false,
      attrs: keep({}),
      parseDOM: [
        { tag: 'span[data-tab]' },
        // Word: <span style='mso-tab-count:1'>
        { tag: 'span', priority: 60, getAttrs: (el) => (/mso-tab-count/i.test(el.getAttribute('style') || '') ? {} : false) },
      ],
      toDOM: () => ['span', { class: 'pm-tab', 'data-tab': '' }, '\t'],
    },
  },

  // Order matters: earlier marks wrap later ones.
  marks: {
    link: {
      attrs: keep({ href: {} }),
      inclusive: false,
      parseDOM: [{ tag: 'a[href]', getAttrs: (el) => (el.hasAttribute('data-story-type') ? false : { href: el.getAttribute('href') }) }],
      toDOM: (mark) => ['a', { href: mark.attrs.href, rel: 'noopener noreferrer', title: mark.attrs.href }, 0],
    },

    // A link to a story item, as the wiki's [[type#id]] links.
    story_link: {
      attrs: keep({ type: {}, id: {} }),
      inclusive: false,
      parseDOM: [
        {
          tag: 'a[data-story-type]',
          priority: 60,
          getAttrs: (el) => ({ type: el.getAttribute('data-story-type'), id: el.getAttribute('data-story-id') }),
        },
      ],
      // The href is the item's wiki page, so the wiki's hover preview finds it
      // (articlePreview.js); a click in the editor only places the cursor.
      toDOM: (mark) => [
        'a',
        { class: 'story-link', href: `/wiki/${mark.attrs.type}/${mark.attrs.id ?? ''}`, 'data-story-type': mark.attrs.type, 'data-story-id': String(mark.attrs.id ?? '') },
        0,
      ],
    },

    // The text uses a character style of the book (its look comes from the
    // book's stylesheet, styles.js), under the text's own marks.
    char_style: {
      attrs: keep({ id: {} }),
      parseDOM: [{ tag: 'span[data-cstyle]', getAttrs: (el) => ({ id: el.getAttribute('data-cstyle') }) }],
      toDOM: (mark) => ['span', { class: 'cs', 'data-cstyle': String(mark.attrs.id ?? '') }, 0],
    },

    font: {
      attrs: keep({ family: {} }),
      parseDOM: [
        { tag: 'span[data-font]', getAttrs: (el) => ({ family: el.getAttribute('data-font') }) },
        { style: 'font-family', getAttrs: (v) => (cssFontFamily(v) ? { family: cssFontFamily(v) } : false) },
      ],
      toDOM: (mark) => ['span', { 'data-font': mark.attrs.family, style: `font-family: ${familyCSS(mark.attrs.family)}` }, 0],
    },

    size: {
      attrs: keep({ pt: {} }),
      parseDOM: [{ style: 'font-size', getAttrs: (v) => (cssFontSizeToPt(v) ? { pt: cssFontSizeToPt(v) } : false) }],
      toDOM: (mark) => ['span', { style: `font-size: ${num(mark.attrs.pt)}pt` }, 0],
    },

    color: {
      attrs: keep({ color: {} }),
      parseDOM: [
        { tag: 'span[data-color]', getAttrs: (el) => (cssColorToHex(el.getAttribute('data-color')) ? { color: cssColorToHex(el.getAttribute('data-color')) } : false) },
        // Black is how most sources write "automatic", so it isn't kept.
        { style: 'color', getAttrs: (v) => (cssColorToHex(v) && cssColorToHex(v) !== '#000000' ? { color: cssColorToHex(v) } : false) },
      ],
      toDOM: (mark) => ['span', { 'data-color': mark.attrs.color, style: `color: ${mark.attrs.color}` }, 0],
    },

    highlight: {
      attrs: keep({ color: { default: '#fff176' } }),
      parseDOM: [
        { tag: 'mark', getAttrs: (el) => ({ color: cssColorToHex(el.style.backgroundColor) || '#fff176' }) },
        { style: 'background-color', getAttrs: (v) => (cssColorToHex(v) && cssColorToHex(v) !== '#ffffff' ? { color: cssColorToHex(v) } : false) },
      ],
      toDOM: (mark) => ['mark', { style: `background-color: ${mark.attrs.color}` }, 0],
    },

    bold: {
      attrs: keep({}),
      parseDOM: [
        { tag: 'strong' },
        // Google Docs wraps a whole paste in <b style="font-weight:normal">.
        { tag: 'b', getAttrs: (el) => el.style.fontWeight !== 'normal' && !/^docs-internal-guid/.test(el.id || '') && null },
        { style: 'font-weight=400', clearMark: (m) => m.type.name === 'bold' },
        { style: 'font-weight=normal', clearMark: (m) => m.type.name === 'bold' },
        { style: 'font-weight', getAttrs: (v) => /^(bold|bolder|[6-9]00)$/.test(String(v).trim()) && null },
      ],
      toDOM: () => ['strong', 0],
    },

    italic: {
      attrs: keep({}),
      parseDOM: [
        { tag: 'i' },
        { tag: 'em' },
        { style: 'font-style=italic' },
        { style: 'font-style=normal', clearMark: (m) => m.type.name === 'italic' },
      ],
      toDOM: () => ['em', 0],
    },

    underline: {
      attrs: keep({}),
      parseDOM: [
        { tag: 'u' },
        { style: 'text-decoration', getAttrs: (v) => /underline/.test(v) && null },
        { style: 'text-decoration-line', getAttrs: (v) => /underline/.test(v) && null },
      ],
      toDOM: () => ['u', 0],
    },

    strike: {
      attrs: keep({}),
      parseDOM: [
        { tag: 's' },
        { tag: 'del' },
        { tag: 'strike' },
        { style: 'text-decoration', getAttrs: (v) => /line-through/.test(v) && null },
        { style: 'text-decoration-line', getAttrs: (v) => /line-through/.test(v) && null },
      ],
      toDOM: () => ['s', 0],
    },

    superscript: {
      attrs: keep({}),
      excludes: 'superscript subscript',
      parseDOM: [{ tag: 'sup' }, { style: 'vertical-align=super' }],
      toDOM: () => ['sup', 0],
    },

    subscript: {
      attrs: keep({}),
      excludes: 'superscript subscript',
      parseDOM: [{ tag: 'sub' }, { style: 'vertical-align=sub' }],
      toDOM: () => ['sub', 0],
    },

    smallcaps: {
      attrs: keep({}),
      parseDOM: [
        { tag: 'span[data-smallcaps]' },
        { style: 'font-variant', getAttrs: (v) => /small-caps/.test(v) && null },
        { style: 'font-variant-caps', getAttrs: (v) => /small-caps/.test(v) && null },
      ],
      toDOM: () => ['span', { 'data-smallcaps': '', style: 'font-variant: small-caps' }, 0],
    },
  },
})

// ---- JSON in and out ----------------------------------------------------------

export const emptyDocJSON = () => ({ type: 'doc', content: [{ type: 'paragraph' }] })

function splitAttrs(type, attrs) {
  if (!attrs || typeof attrs !== 'object') return undefined
  const known = {}
  let extra = null
  for (const [k, v] of Object.entries(attrs)) {
    if (k === 'extra') continue
    if (k in type.attrs) known[k] = v
    else (extra ||= {})[k] = v
  }
  if (attrs.extra && typeof attrs.extra === 'object') extra = { ...attrs.extra, ...extra }
  if (extra && 'extra' in type.attrs) known.extra = extra
  return known
}

const textOf = (json) =>
  json.type === 'text' ? String(json.text || '') : Array.isArray(json.content) ? json.content.map(textOf).join('') : ''

function normalise(json, inline) {
  if (!json || typeof json !== 'object') return null
  const type = schema.nodes[json.type]
  if (!type) {
    // A node this editor doesn't know: its text, so nothing written is lost.
    const text = textOf(json)
    if (inline) return text ? { type: 'text', text } : null
    return { type: 'paragraph', content: text ? [{ type: 'text', text }] : undefined }
  }
  if (type.name === 'text') {
    if (!json.text) return null
    const marks = (json.marks || [])
      .filter((m) => m && schema.marks[m.type])
      .map((m) => ({ type: m.type, attrs: splitAttrs(schema.marks[m.type], m.attrs) }))
    return { type: 'text', text: String(json.text), marks: marks.length ? marks : undefined }
  }
  const out = { type: type.name, attrs: splitAttrs(type, json.attrs) }
  if (Array.isArray(json.content) && !type.isLeaf) {
    const content = json.content.map((c) => normalise(c, type.inlineContent)).filter(Boolean)
    if (content.length) out.content = content
  }
  return out
}

// A stored document → a ProseMirror node. Anything unreadable gives an
// empty document rather than a broken page.
export function docFromJSON(json) {
  const norm = normalise(json && json.type === 'doc' ? json : emptyDocJSON(), false)
  if (!norm.content) norm.content = [{ type: 'paragraph' }]
  try {
    const doc = schema.nodeFromJSON(norm)
    doc.check()
    return doc
  } catch (e) {
    // Content in a place the schema doesn't allow: keep each block's text.
    const blocks = []
    norm.content.forEach(function walk(n) {
      if (n.type === 'paragraph' || n.type === 'heading' || n.type === 'dialogue') {
        blocks.push({ type: 'paragraph', content: textOf(n) ? [{ type: 'text', text: textOf(n) }] : undefined })
      } else (n.content || []).forEach(walk)
    })
    return schema.nodeFromJSON({ type: 'doc', content: blocks.length ? blocks : [{ type: 'paragraph' }] })
  }
}

function compactAttrs(type, attrs) {
  if (!attrs) return undefined
  const out = {}
  for (const [k, v] of Object.entries(attrs)) {
    if (k === 'extra') continue
    const spec = type.attrs[k]
    // Only empty values are left out (null, false), so a reader in another
    // language never has to know the schema's other defaults.
    if (spec && spec.hasDefault && spec.default === v && (v === null || v === false)) continue
    out[k] = v
  }
  if (attrs.extra) Object.assign(out, attrs.extra)
  return Object.keys(out).length ? out : undefined
}

function compact(json) {
  if (json.type === 'text') {
    if (json.marks) json.marks = json.marks.map((m) => ({ type: m.type, attrs: compactAttrs(schema.marks[m.type], m.attrs) }))
    for (const m of json.marks || []) if (!m.attrs) delete m.attrs
    return json
  }
  const attrs = compactAttrs(schema.nodes[json.type], json.attrs)
  if (attrs) json.attrs = attrs
  else delete json.attrs
  if (json.content) json.content = json.content.map(compact)
  return json
}

// A ProseMirror node → the JSON stored. Attrs that are null or false are left out;
// unknown ones go back where they were.
export function docToJSON(doc) {
  return compact(doc.toJSON())
}
