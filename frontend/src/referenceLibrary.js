// The reference library (backend/references.go): real-world pictures kept
// for inspiration, one library for every story, so its calls are global
// (/api/references), not under the story's prefix. Story designs (culture
// elements) point at references by id.
import { fetchKept } from './offline/kept.js'
import { ref } from 'vue'
import { t, tr } from './i18n'

// Messages the library's calls can give; listed so they get translated.
void [
  tr('reference not found'), tr('title is required'), tr('that text is too long'), tr('unknown kind'),
  tr('the source must be a web address (http or https)'), tr('invalid tag list'), tr('too many tags'),
  tr('invalid picture list'), tr('pictures must be PNG, JPEG, GIF, WebP or AVIF'),
  tr('that picture is too large (over 32 MB)'), tr('those pictures are too large'), tr('invalid form data'),
  tr('failed to save the reference'), tr('failed to delete the reference'), tr('failed to load the references'),
]

// The kinds of reference; clothing first. The same keys as referenceKinds
// in backend/references.go.
export const REFERENCE_KINDS = [
  { id: 'clothing', label: tr('Clothing') },
  { id: 'armor', label: tr('Armour') },
  { id: 'weapon', label: tr('Weapons') },
  { id: 'hair', label: tr('Hair') },
  { id: 'jewelry', label: tr('Jewellery') },
  { id: 'architecture', label: tr('Architecture') },
  { id: 'other', label: tr('Other') },
]

export const referenceLibrary = ref([])
let loading = null

async function call(url, options, fallback) {
  let res
  try {
    res = await fetchKept(url, options) // the kept list when the server can't be reached
  } catch (e) {
    throw new Error(t("can't reach the server"))
  }
  if (!res.ok) {
    let msg = ''
    try {
      msg = (await res.text()).trim()
    } catch (e) {
      /* no body */
    }
    throw new Error(t(msg || fallback))
  }
  return res.json()
}

// The library, loaded once (force: again).
export function loadReferences(force = false) {
  if (!loading || force) {
    loading = call('/api/references', undefined, 'failed to load the references')
      .then((list) => (referenceLibrary.value = Array.isArray(list) ? list : []))
      .catch((e) => {
        loading = null
        throw e
      })
  }
  return loading
}

const newestFirst = (a, b) => (a.created_at < b.created_at ? 1 : a.created_at > b.created_at ? -1 : b.id - a.id)

// formData: title, kind, source_url, description, tags (JSON), pictures
// (files), remove_pictures and order (JSON picture ids).
export async function saveReference(id, formData) {
  const saved = await call(
    id ? `/api/references/${id}` : '/api/references',
    { method: id ? 'PUT' : 'POST', body: formData },
    'failed to save the reference',
  )
  referenceLibrary.value = [saved, ...referenceLibrary.value.filter((r) => r.id !== saved.id)].sort(newestFirst)
  return saved
}

export async function deleteReference(id) {
  await call(`/api/references/${id}`, { method: 'DELETE' }, 'failed to delete the reference')
  referenceLibrary.value = referenceLibrary.value.filter((r) => r.id !== id)
}

// { "<reference id>": [{ story_id, story, culture_id, culture, element_id, element }] }
export const referenceUses = () => call('/api/references/used', undefined, 'failed to load the references')
