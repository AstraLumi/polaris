// Live gravity off the page's thread (gravityEngine.js sends the requests).
import { runGravity } from './gravityEngine.js'

let run = null
self.onmessage = (e) => {
  const msg = e.data
  if (msg.type === 'init') {
    run = runGravity(msg.bodies, msg.ids, msg.options)
    return
  }
  if (msg.type === 'reset') return self.postMessage(run.reset(msg.shapes))
  if (msg.type === 'reset-then-advance') run.reset(false)
  const state = run.advance(msg.target, msg.shapes)
  self.postMessage(state, [state.rel.buffer])
}
