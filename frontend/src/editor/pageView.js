// The page view's book (docs/EDITOR.md, "Client"): every chapter drawn the
// way the editor draws it (the schema's toDOM, the styles' stylesheet),
// marked so a click finds its chapter and block, then laid out in pages by
// paged.js, which is loaded only here. The layout's rules come from
// bookLayout.js (pagedCSS).
import { DOMSerializer } from 'prosemirror-model'
import { schema, docFromJSON, ownPara } from './schema.js'
import { paraDeclarations } from './css.js'
import { bookPlan, pageName, tocEntries, normaliseLayout } from './bookLayout.js'
import { noteKind, noteLabel } from './footnotes.js'

const textOf = (n) =>
  n.type === 'text' ? n.text || '' : n.type === 'hard_break' || n.type === 'tab' ? ' ' : n.type === 'footnote' ? '' : (n.content || []).map(textOf).join('')

// The schema's serializer, but a note (footnote or endnote, stage 5) is
// its number (sup.pv-fn-call) followed by its text (span.pv-fn: the number
// again, then each paragraph as a span.pv-fn-p, a block). Footnotes stay
// where they are for paged.js (float: footnote, pagedCSS); endnotes, and
// every note of an online post, are gathered at the chapter's end
// (gatherNotes). counts: { footnote, endnote }, set to 0 for each chapter.
function bookSerializer(doc, counts) {
  const base = DOMSerializer.fromSchema(schema)
  let serializer = null
  const footnote = (node) => {
    const kind = noteKind(node.attrs.kind)
    const label = noteLabel(kind, ++counts[kind])
    const wrap = doc.createElement('span')
    wrap.className = 'pv-fn-wrap'
    const call = wrap.appendChild(doc.createElement('sup'))
    call.className = `pv-fn-call pv-${kind}`
    call.textContent = label
    const body = wrap.appendChild(doc.createElement('span'))
    body.className = `pv-fn pv-${kind}`
    body.dataset.label = label
    node.forEach((p, _o, i) => {
      const line = body.appendChild(doc.createElement('span'))
      line.className = 'pv-fn-p'
      const css = paraDeclarations(ownPara(p.attrs)).join('; ')
      if (css) line.setAttribute('style', css)
      if (i === 0) {
        const num = line.appendChild(doc.createElement('span'))
        num.className = 'pv-fn-num'
        num.textContent = label
      }
      line.appendChild(serializer.serializeFragment(p.content, { document: doc }))
    })
    return wrap
  }
  serializer = new DOMSerializer({ ...base.nodes, footnote }, base.marks)
  return serializer
}

// Moves the notes of a chapter (endnotes when paged, every note otherwise:
// footnotes first, then endnotes) into a list at its end, div.pv-notes.
function gatherNotes(section, paged, doc) {
  const found = [...section.querySelectorAll(paged ? '.pv-fn.pv-endnote' : '.pv-fn')]
  if (!found.length) return
  const order = paged ? found : [...found.filter((n) => n.classList.contains('pv-footnote')), ...found.filter((n) => n.classList.contains('pv-endnote'))]
  const list = doc.createElement('div')
  list.className = 'pv-notes'
  for (const n of order) {
    const item = list.appendChild(doc.createElement('div'))
    item.className = `pv-note ${n.classList.contains('pv-endnote') ? 'pv-endnote' : 'pv-footnote'}`
    item.dataset.ch = n.dataset.ch
    item.dataset.blk = n.dataset.blk
    while (n.firstChild) item.appendChild(n.firstChild)
    n.remove()
  }
  section.appendChild(list)
}

// The book as HTML: { fragment, plan, blocks (how many top-level blocks,
// for the progress) }. chapters: [{ id, title, doc (JSON) }]. Each
// top-level block carries data-ch (its chapter), data-blk (its index in
// the chapter's doc) and data-gi (its index in the book). With paged
// false (an online post) nothing is laid out in pages.
export function buildBook(chapters, settings, { paged = true, doc = document } = {}) {
  const counts = { footnote: 0, endnote: 0 }
  const serializer = bookSerializer(doc, counts)
  const layout = normaliseLayout(settings?.layout)
  const plan = bookPlan(settings, chapters)
  const fragment = doc.createDocumentFragment()
  let gi = 0
  chapters.forEach((ch, ci) => {
    const pl = plan[ci]
    counts.footnote = counts.endnote = 0
    const node = docFromJSON(ch.doc)
    const h1 = (ch.doc?.content || []).find((n) => n?.type === 'heading' && (+n.attrs?.level || 1) === 1)
    const section = doc.createElement('section')
    section.className = `pv-chapter pv-doc write-doc${paged ? ` pv-${pageName(pl)}` : ' is-online'}`
    section.id = `pv-ch-${ch.id}`
    section.dataset.title = (h1 ? textOf(h1).replace(/\s+/g, ' ').trim() : '') || ch.title || ''
    section.dataset.number = pl.number
    section.dataset.ch = String(ch.id)
    let leading = paged // full-page pictures at a chapter's start go before it
    let hoisted = false
    node.forEach((block, _off, bi) => {
      let el
      if (block.type.name === 'toc') el = tocElement(block, chapters, plan, layout, doc)
      else el = serializer.serializeNode(block, { document: doc })
      if (el.nodeType !== 1) {
        const wrap = doc.createElement('div')
        wrap.appendChild(el)
        el = wrap
      }
      el.dataset.ch = String(ch.id)
      el.dataset.blk = String(bi)
      el.dataset.gi = String(gi++)
      for (const n of el.querySelectorAll('.pv-fn')) {
        n.dataset.ch = String(ch.id)
        n.dataset.blk = String(bi)
      }
      if (block.type.name === 'heading') el.id = `pv-h-${ch.id}-${bi}`
      if (block.type.name === 'table') headerRowsToHead(el, doc)
      if (block.type.name === 'page_break') {
        el.className = paged ? 'pv-page-break' : 'page-break'
      }
      const full = paged && block.type.name === 'image' && block.attrs.fullPage
      if (full) el.classList.add('pv-full')
      if (full && leading) {
        // Before the chapter, in a wrapper drawn like the chapter's text.
        const wrap = doc.createElement('div')
        wrap.className = 'pv-full pv-doc write-doc'
        el.classList.remove('pv-full')
        wrap.appendChild(el)
        wrap.dataset.ch = String(ch.id)
        wrap.dataset.blk = String(bi)
        if (fragment.childNodes.length) wrap.classList.add('pv-later')
        fragment.appendChild(wrap)
        hoisted = true
        return
      }
      leading = false
      section.appendChild(el)
    })
    gatherNotes(section, paged, doc)
    if (fragment.childNodes.length && !hoisted) {
      section.classList.add('pv-later')
    }
    if (!section.childNodes.length) {
      const p = doc.createElement('p')
      p.dataset.ch = String(ch.id)
      p.dataset.blk = '0'
      section.appendChild(p)
    }
    fragment.appendChild(section)
  })
  return { fragment, plan, blocks: gi }
}

// A table's header rows (its leading rows of header cells) go in a thead,
// which repeatingTables repeats on each page the table runs onto.
export function headerRowsToHead(table, doc = document) {
  const body = table.querySelector(':scope > tbody')
  if (!body) return
  const rows = []
  for (const tr of body.children) {
    if (tr.children.length && [...tr.children].every((c) => c.tagName === 'TH')) rows.push(tr)
    else break
  }
  // A table of header rows only has nothing to repeat them over.
  if (!rows.length || rows.length === body.children.length) return
  const head = doc.createElement('thead')
  for (const tr of rows) head.appendChild(tr)
  table.insertBefore(head, body)
}

// A paged.js handler: a table split across pages gets its columns (the
// colgroup, which holds their widths) and its header rows (thead) again on
// each page it continues on. paged.js rebuilds a split table from its
// ancestors alone, so without this the next page's part has neither.
// Header rows the page broke inside are not repeated (they continue as they are).
function repeatingTables(Handler) {
  return class RepeatingTables extends Handler {
    constructor(chunker, polisher, caller) {
      super(chunker, polisher, caller)
      this.split = [] // [{ ref, head }]
    }

    afterPageLayout(pageElement, page, breakToken, chunker) {
      this.source = chunker.source
      this.split = []
      let node = breakToken?.node
      if (!node) return
      if (node.nodeType !== 1) node = node.parentElement
      let inHead = false
      for (let el = node; el; el = el.parentElement) {
        if (el.tagName === 'THEAD') inHead = true
        if (el.tagName === 'TABLE' && el.dataset.ref) this.split.push({ ref: el.dataset.ref, head: !inHead })
      }
    }

    layout(rendered) {
      for (const { ref, head } of this.split) {
        const table = rendered.querySelector(`table[data-ref="${ref}"]`)
        if (!table || !table.hasAttribute('data-split-from') || table.dataset.repeated) continue
        const source = this.source?.querySelector(`table[data-ref="${ref}"]`)
        if (!source) continue
        table.dataset.repeated = '1'
        const thead = source.querySelector(':scope > thead')
        if (head && thead && !table.querySelector(':scope > thead')) table.insertBefore(thead.cloneNode(true), table.firstChild)
        const cols = source.querySelector(':scope > colgroup')
        if (cols && !table.querySelector(':scope > colgroup')) table.insertBefore(cols.cloneNode(true), table.firstChild)
      }
    }
  }
}

let handlersRegistered = false

// A table of contents with its entries; the page numbers come from
// pagedCSS's target-counter rules.
function tocElement(node, chapters, plan, layout, doc) {
  const nav = doc.createElement('nav')
  nav.className = `pv-toc${layout.toc_dots ? ' has-dots' : ''}`
  const title = doc.createElement('p')
  title.className = 'pv-toc-title'
  title.textContent = node.attrs.title ?? ''
  nav.appendChild(title)
  const list = doc.createElement('ol')
  for (const e of tocEntries(chapters, plan, Math.min(3, Math.max(1, node.attrs.levels || 1)))) {
    const li = doc.createElement('li')
    li.className = `pv-toc-l${e.level}`
    const a = doc.createElement('a')
    a.href = e.block == null ? `#pv-ch-${e.chapter}` : `#pv-h-${e.chapter}-${e.block}`
    a.className = e.front ? 'pv-n-front' : 'pv-n-main'
    const text = doc.createElement('span')
    text.className = 'pv-toc-text'
    text.textContent = e.text
    const dots = doc.createElement('span')
    dots.className = 'pv-toc-dots'
    a.append(text, dots)
    li.appendChild(a)
    list.appendChild(li)
  }
  nav.appendChild(list)
  return nav
}

// Lays the book out in pages inside renderTo. onPage(n, lastGi) after each
// page. Gives { pages, cancel(), destroy() }: cancel stops a layout under
// way, destroy also takes paged.js's styles out of the document.
export async function layOutBook(fragment, css, renderTo, { onPage } = {}) {
  const { Previewer, Handler, registerHandlers } = await import('pagedjs')
  if (!handlersRegistered) {
    registerHandlers(repeatingTables(Handler))
    handlersRegistered = true
  }
  // Pictures loaded (and decoded) before the layout measures them; a
  // picture first seen in the hidden pages would otherwise be left out of
  // the print until it had been on screen.
  const srcs = [...new Set([...fragment.querySelectorAll('img[src]')].map((i) => i.getAttribute('src')))]
  await Promise.all(
    srcs.map((src) => {
      const img = new Image()
      img.src = src
      return img.decode().catch(() => {})
    }),
  )
  for (const img of fragment.querySelectorAll('img')) {
    img.loading = 'eager'
    img.decoding = 'sync'
  }
  const previewer = new Previewer()
  let n = 0
  let stopped = false
  // After each page is laid out (its text is in it by then).
  previewer.chunker.on('renderedPage', (page) => {
    n++
    const marks = page.element?.querySelectorAll?.('[data-gi]')
    const last = marks && marks.length ? +marks[marks.length - 1].dataset.gi : -1
    onPage?.(n, last)
  })
  const handle = {
    pages: 0,
    // paged.js starts a stopped layout again; this makes the next round end.
    cancel() {
      stopped = true
      previewer.chunker.render = async () => ({ done: true })
      previewer.chunker.stop()
    },
    destroy() {
      handle.cancel()
      try {
        previewer.polisher.destroy()
      } catch (e) {
        /* nothing was inserted yet */
      }
    },
    previewer,
    // paged.js restarts the page counter with counter-reset on the page,
    // which current browsers keep to that page (the next one counts on from
    // the old number). Once laid out, the same restarts as counter-set on the
    // counter all pages share. (Not during the layout: paged.js reads the
    // pages' counter-reset to number the table of contents.)
    restartsCSS() {
      const out = []
      let rules = []
      try {
        rules = [...(previewer.polisher.styleSheet?.cssRules || [])]
      } catch (e) {
        /* no rules */
      }
      for (const r of rules) {
        const m = /\[data-page-number="(\d+)"\]\s*\{\s*counter-increment:\s*none;\s*counter-reset:\s*page (-?\d+)/.exec(r.cssText)
        if (m) out.push(`.pv-pages .pagedjs_page[data-page-number="${m[1]}"] { counter-reset: none; counter-set: page ${m[2]}; counter-increment: none; }`)
      }
      return out.join('\n')
    },
  }
  const done = previewer.preview(fragment, [{ [`${location.origin}/polaris-book.css`]: css }], renderTo).then((flow) => {
    handle.pages = flow?.total ?? n
    return !stopped
  })
  return { handle, done }
}
