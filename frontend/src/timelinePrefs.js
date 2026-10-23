// How the timeline is drawn, remembered on this device (Settings → Language &
// look, read by TimelineView): 'snake' (the line turns at each row's end and
// comes back, every other row right to left) or 'ltr' (every row left to
// right, like lines of text).

import { ref, watch } from 'vue'

const KEY = 'polaris-timeline-rows'
function read() {
  try {
    return localStorage.getItem(KEY) === 'ltr' ? 'ltr' : 'snake'
  } catch {
    return 'snake'
  }
}

export const timelineRows = ref(read())
watch(timelineRows, (v) => {
  try {
    localStorage.setItem(KEY, v)
  } catch {
    // Not remembered in a private window.
  }
})
