// Making (or renewing) a story's copy for reading offline, in the browser
// (docs/DEVELOPMENT.md, "Reading offline"). Loaded only when asked for.
import { ref } from 'vue'
import { send } from '../request.js'
import { t } from '../i18n'
import { connected } from './connection.js'
import { keptStories, markKept, dataCache, SHARED } from './kept.js'
import { keepFiles, askPersistence } from './caches.js'
import { copyStory } from './storyCopy.js'

// The copy being made: { story, done, total }, or null.
export const copying = ref(null)

// Has the service worker keep every file of the app. Answers how many are
// missing; null without a service worker (the dev server).
async function keepAppFiles() {
  try {
    const worker = (await navigator.serviceWorker?.getRegistration())?.active
    if (!worker) return null
    return await new Promise((resolve) => {
      const channel = new MessageChannel()
      const timer = setTimeout(() => resolve(null), 120000)
      channel.port1.onmessage = (e) => {
        clearTimeout(timer)
        resolve(e.data?.missing ?? 0)
      }
      worker.postMessage({ type: 'keep-app' }, [channel.port2])
    })
  } catch (e) {
    return null
  }
}

export async function keepStory(story, name = '') {
  if (copying.value) return
  if (!connected.value) throw new Error(t("can't reach the server"))
  const before = keptStories.value[story]
  // Marked first: from here on `send` keeps this story's answers.
  markKept(story, before || { at: 0, name, missing: 0 })
  copying.value = { story, done: 0, total: 0 }
  try {
    await Promise.all(SHARED.map((u) => send(u).catch(() => null)))
    const get = async (path) => {
      try {
        const res = await send(`/api/s/${story}/${path}`)
        return { reached: true, text: res.ok ? await res.text() : null }
      } catch (e) {
        return { reached: false }
      }
    }
    let answers = 0
    const out = await copyStory({
      get,
      onProgress: (done, total) => {
        answers = total
        copying.value = { story, done, total }
      },
    })
    // The connection went: what was kept before stays as it was.
    if (!connected.value || out.failed.length > out.asked.length / 2) throw new Error(t("can't reach the server"))
    const lost = await keepFiles(dataCache(story), out.files, (done, total) => {
      copying.value = { story, done: answers + done, total: answers + total }
    })
    const appMissing = await keepAppFiles()
    askPersistence()
    markKept(story, { at: Date.now(), name: name || before?.name || '', missing: out.failed.length + lost.length + (appMissing || 0) })
  } finally {
    copying.value = null
  }
}
