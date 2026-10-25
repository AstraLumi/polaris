// The table of contents as the editor draws it (a ProseMirror node view):
// a box with its title and the book's chapters, without page numbers,
// which only the page view knows. getEntries() gives [{ title }] (the
// chapters that are listed); hint is the line under them.
export function tocNodeView(getEntries, hint = '') {
  return (node) => {
    const dom = document.createElement('nav')
    dom.className = 'pm-toc'
    dom.setAttribute('data-toc', '')
    dom.contentEditable = 'false'
    let current = node
    const render = () => {
      dom.setAttribute('data-levels', String(current.attrs.levels || 1))
      dom.setAttribute('data-title', current.attrs.title ?? '')
      dom.textContent = ''
      const title = document.createElement('p')
      title.className = 'pm-toc-title'
      title.textContent = current.attrs.title ?? ''
      dom.appendChild(title)
      const list = document.createElement('ol')
      for (const e of getEntries()) {
        const li = document.createElement('li')
        li.textContent = e.title
        list.appendChild(li)
      }
      dom.appendChild(list)
      if (hint) {
        const p = document.createElement('p')
        p.className = 'pm-toc-hint'
        p.textContent = hint
        dom.appendChild(p)
      }
    }
    render()
    return {
      dom,
      update(next) {
        if (next.type !== current.type) return false
        current = next
        render()
        return true
      },
      ignoreMutation: () => true,
    }
  }
}
