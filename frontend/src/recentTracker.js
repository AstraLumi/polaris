// Fills the "recently opened" list (recent.js) from the router, so the item
// pages don't have to know about it: when a character, event, chapter or wiki
// article page is reached, the name it publishes as pageTitle (once it has
// loaded the item) is what gets remembered. Pages that fail to load never
// publish a name, so a deleted item is never recorded.

import { ref, watch } from 'vue'
import { pageTitle } from './navigation'
import { activeStoryId } from './stories'
import { wikiApi } from './wiki'
import { chaptersApi } from './api'
import { loadRecent, saveRecent, pushRecent, dropRecent, recentKey } from './recent'

export const recentItems = ref(loadRecent(activeStoryId))

function commit(list) {
  recentItems.value = list
  saveRecent(activeStoryId, list)
}

// Forget items that no longer exist, given as a list of { type, id }.
export function forgetRecent(gone) {
  let list = recentItems.value
  for (const g of gone) list = dropRecent(list, g.type, g.id)
  if (list.length !== recentItems.value.length) commit(list)
}

// Pictures of the remembered items, by recentKey, known after checkRecent.
export const recentPictures = ref({})

// Checks the list against the story: items that were deleted since are
// dropped, renamed ones get their new name, and the pictures are collected.
// If the story can't be read the list is left as it is.
export async function checkRecent() {
  if (!recentItems.value.length) return
  try {
    const [items, chapters] = await Promise.all([wikiApi.list(), chaptersApi.list()])
    const now = new Map()
    const pictures = {}
    for (const i of items) {
      now.set(`${i.type}:${i.id}`, i.name)
      if (i.picture) pictures[`${i.type}:${i.id}`] = i.picture
    }
    for (const c of chapters.chapters || []) now.set(`chapter:${c.id}`, c.title)
    const list = recentItems.value.filter((e) => now.has(recentKey(e))).map((e) => ({ ...e, name: now.get(recentKey(e)) || e.name }))
    recentPictures.value = pictures
    if (JSON.stringify(list) !== JSON.stringify(recentItems.value)) commit(list)
  } catch (e) {
    /* offline or the server is busy: keep the list */
  }
}

// What a route shows: the item, and whether the page itself names it (the
// main page of the item) or only belongs to it (its family tree, its edit form).
function itemOf(to) {
  const id = Number(to.params.id)
  if (!Number.isInteger(id)) return null
  switch (to.name) {
    case 'character-view':
      return { type: 'character', id, own: true }
    case 'family-tree':
    case 'version-list':
    case 'version-compare':
    case 'version-edit':
      return { type: 'character', id, own: false }
    case 'event-view':
      return { type: 'event', id, own: true }
    case 'event-edit':
      return { type: 'event', id, own: false }
    case 'chapter':
      return { type: 'chapter', id, own: true }
    case 'wiki-article':
      return to.params.type ? { type: to.params.type, id, own: true } : null
    default:
      return null
  }
}

let pending = null

export function trackRecent(router) {
  router.afterEach((to, from, failure) => {
    pending = null
    if (failure) return
    const item = itemOf(to)
    if (!item) return
    if (item.own) {
      pending = item
      return
    }
    // A page of an item already remembered brings it back to the top.
    const known = recentItems.value.find((e) => recentKey(e) === recentKey(item))
    if (known) commit(pushRecent(recentItems.value, { ...known, at: Date.now() }))
  })

  // Sync, so the name is read before the next navigation clears it.
  watch(
    pageTitle,
    (name) => {
      if (!name || !pending) return
      const { type, id } = pending
      pending = null
      commit(pushRecent(recentItems.value, { type, id, name, at: Date.now() }))
    },
    { flush: 'sync' },
  )
}
