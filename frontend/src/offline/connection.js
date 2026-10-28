// Whether the server can be reached (docs/EDITOR.md, "Offline writing"):
// the browser's online/offline events, and calls that failed to reach it
// (the browser can say "online" with the server down). While it can't be
// reached, /api/health is asked every little while; `whenBack` callbacks run
// when it answers again.
import { ref, computed } from 'vue'

const browserOnline = ref(typeof navigator === 'undefined' || navigator.onLine !== false)
const reachable = ref(true)
export const connected = computed(() => browserOnline.value && reachable.value)

const listeners = new Set()
export function whenBack(fn) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

let timer = null
const PROBE_EVERY = 15000

function back() {
  const was = connected.value
  reachable.value = true
  browserOnline.value = true
  clearTimeout(timer)
  timer = null
  if (!was) for (const fn of [...listeners]) Promise.resolve().then(fn).catch(() => {})
}

export async function probe() {
  try {
    const res = await fetch('/api/health', { cache: 'no-store' })
    if (res.ok) return back(), true
  } catch (e) {
    /* still unreachable */
  }
  return false
}

function schedule() {
  clearTimeout(timer)
  timer = setTimeout(async () => {
    if (!(await probe())) schedule()
  }, PROBE_EVERY)
}

// A call couldn't reach the server (status 0) or reached it.
export function reportReach(ok) {
  if (ok) {
    if (!connected.value) back()
    return
  }
  if (reachable.value) {
    reachable.value = false
    schedule()
  }
}

if (typeof window !== 'undefined') {
  window.addEventListener('offline', () => {
    browserOnline.value = false
    reachable.value = false
    schedule()
  })
  window.addEventListener('online', () => {
    browserOnline.value = true
    probe().then((ok) => ok || schedule())
  })
}
