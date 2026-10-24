import { shallowRef, ref } from 'vue'
import { wikiApi, makeLinkResolver, linksForEditing } from './wiki'
import { linksAsText } from './wikiLinks.js'

// The names behind the [[links]] in the items' own texts (a character's
// description, bio and speech, the description of a location, faction,
// event, spell, gear or creature). They are stored by id (backend
// wikilinks.go, itemLinkColumns), so the pages that show them need the
// wiki's article list: loaded here once for every page and row that shows
// such a text, and again when a page opens after a few seconds (a rename may
// have happened meanwhile).
//
// Showing one: linkText(text), never the text itself. Editing one:
// linkField(stored), a ref holding the text with its links by name, which
// the server stores by id again on save (an unchanged save gives back the
// same bytes).

const resolver = shallowRef(null)
let pending = null
let loadedAt = 0
const FRESH_MS = 3000

// Loads the article list unless it was loaded a moment ago (or force).
// Resolves to the resolver (null when the list can't be read).
export function loadLinkNames({ force = false } = {}) {
  if (pending) return pending
  if (!force && resolver.value && Date.now() - loadedAt < FRESH_MS) return Promise.resolve(resolver.value)
  pending = wikiApi
    .list()
    .then((items) => {
      resolver.value = makeLinkResolver(items)
      loadedAt = Date.now()
      return resolver.value
    })
    .catch(() => resolver.value)
    .finally(() => {
      pending = null
    })
  return pending
}

// For a page or row that shows such texts (in its setup): starts loading the
// names. linkText re-renders it once they are in.
export function useLinkNames() {
  loadLinkNames()
  return resolver
}

// The text as a reader sees it: links as their names or text (linksAsText).
export function linkText(text) {
  return linksAsText(text, resolver.value)
}

// The text with its stored links by name, as a form shows it.
export function linksByName(text) {
  return linksForEditing(text, resolver.value)
}

// A form field for a stored text, shown with its links by name. Fresh names
// are fetched; when they arrive the field is filled in again with them,
// unless it was changed meanwhile.
export function linkField(stored) {
  const raw = String(stored ?? '')
  const field = ref(linksByName(raw))
  if (raw.includes('[[')) {
    const shown = field.value
    loadLinkNames({ force: true }).then(() => {
      if (field.value === shown) field.value = linksByName(raw)
    })
  }
  return field
}

// For a page that fills its form after loading: the stored text by name,
// once fresh names are in.
export async function loadLinksByName(stored) {
  const raw = String(stored ?? '')
  if (raw.includes('[[')) await loadLinkNames({ force: true })
  return linksByName(raw)
}
