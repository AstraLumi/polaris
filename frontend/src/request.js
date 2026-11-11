// The one fetch every call to the server goes through (api.js, stories.js,
// the wiki, the calendar…), so they all fail the same way: with no
// connection (or the server down) the error says so in the reader's
// language instead of the browser's "Failed to fetch", and a read that gets
// no answer gives up after a minute rather than leave a page loading for
// ever. Saves and uploads are never timed: a large picture on a slow line
// may rightly take longer.
//
// Not through here: the writing editor's calls (editor/api.js, which keep
// the text in the browser when the server can't be reached), the fonts
// library's, file downloads, and what the service worker and the web
// workers fetch.
import { t, tr } from './i18n'
import { leaving } from './undoDelete'

const READ_TIMEOUT = 60000

export async function send(url, options = {}) {
  const read = !options.method || options.method === 'GET'
  // The tab is closing (undoDelete.js): the request is to outlive the page.
  if (leaving.now && !read) options = { ...options, keepalive: true }
  try {
    return await fetch(url, read && !options.signal ? { ...options, signal: AbortSignal.timeout(READ_TIMEOUT) } : options)
  } catch (e) {
    if (e?.name === 'AbortError') throw e // the caller's own signal
    const slow = e?.name === 'TimeoutError'
    throw new Error(t(slow ? tr('the server took too long to answer') : tr("can't reach the server")))
  }
}
