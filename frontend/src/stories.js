// Stories: which one is open on this device, and the calls that manage them.
//
// The open story is remembered in localStorage and fixed for the life of a page
// load: switching stories reloads the app (openStory), so no state from one
// story can linger in another. Every story-scoped API call goes through
// STORY_API (/api/s/<id>).

import { ref, computed } from 'vue'
import { t, tr } from './i18n'

const KEY = 'polaris-story'

function readSaved() {
  try {
    return localStorage.getItem(KEY) || ''
  } catch (e) {
    return ''
  }
}

export const activeStoryId = readSaved()
export const STORY_API = activeStoryId ? `/api/s/${activeStoryId}` : ''

export function rememberStory(id) {
  try {
    localStorage.setItem(KEY, id)
  } catch (e) {
    /* can't remember it; the picker will simply show again */
  }
}

export function forgetStory() {
  try {
    localStorage.removeItem(KEY)
  } catch (e) {
    /* nothing to forget */
  }
}

// Open a story: remember it and start the app fresh on its home page.
export function openStory(id) {
  rememberStory(id)
  window.location.assign('/')
}

export const stories = ref([])
export const storiesLoaded = ref(false)
export const currentStory = computed(() => stories.value.find((s) => s.id === activeStoryId) || null)

// Messages the server can send for story actions. Listed so they get
// translated (the server's text is looked up as is).
void [
  tr('a story needs a name'),
  tr('that story name is too long (60 characters at most)'),
  tr('the icon must be an image (png, jpg, gif, webp or svg)'),
  tr('failed to save the icon'),
  tr("the story to copy from doesn't exist"),
  tr("couldn't copy the Character Assets"),
  tr("type the story's name exactly to delete it"),
  tr('story not found'),
  tr('invalid form data'),
  tr("that file isn't a valid story export"),
  tr('that story export has too many files'),
  tr("that file isn't a Polaris story export"),
  tr('that export was made by a newer version of Polaris'),
  tr('that story export is too large'),
  tr('the story database is damaged'),
  tr("that file isn't a Polaris story database"),
  tr("that export was made by a different version of Polaris and can't be opened"),
  tr("couldn't read the upload"),
  tr('choose a story .zip to import'),
  tr('import failed'),
  tr('export failed'),
  tr('failed to load stories'),
  tr('failed to open story'),
]

async function failure(res, fallback) {
  let text = ''
  try {
    text = (await res.text()).trim()
  } catch (e) {
    /* no body */
  }
  return new Error(t(text && text.length < 200 ? text : fallback))
}

export async function loadStories() {
  const res = await fetch('/api/stories')
  if (!res.ok) throw await failure(res, 'failed to load stories')
  stories.value = await res.json()
  storiesLoaded.value = true
  return stories.value
}

function storyForm({ name, file, iconChoice, copyFrom }) {
  const fd = new FormData()
  if (name !== undefined) fd.append('name', name)
  if (file) fd.append('icon', file)
  else if (iconChoice) fd.append('icon_choice', iconChoice)
  if (copyFrom) fd.append('copy_assets_from', copyFrom)
  return fd
}

export async function createStory(fields) {
  const res = await fetch('/api/stories', { method: 'POST', body: storyForm(fields) })
  if (!res.ok) throw await failure(res, "Couldn't create that story.")
  return res.json()
}

export async function updateStory(id, fields) {
  const res = await fetch(`/api/stories/${id}`, { method: 'PUT', body: storyForm(fields) })
  if (!res.ok) throw await failure(res, "Couldn't save that story.")
  return res.json()
}

export async function deleteStory(id, confirmName) {
  const res = await fetch(`/api/stories/${id}?confirm=${encodeURIComponent(confirmName)}`, { method: 'DELETE' })
  if (!res.ok) throw await failure(res, "Couldn't delete that story.")
}

export async function importStory(file) {
  const fd = new FormData()
  fd.append('file', file)
  const res = await fetch('/api/stories/import', { method: 'POST', body: fd })
  if (!res.ok) throw await failure(res, "Couldn't import that file.")
  return res.json()
}

export const storyExportUrl = (id) => `/api/stories/${id}/export`
