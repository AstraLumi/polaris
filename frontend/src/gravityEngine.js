// Live gravity (orbits.js createGravitySim) run where it can't hold up the
// page: in a worker (gravity.worker.js) when there is one, else right here
// (the tests, old browsers). Either way it answers the same requests with
// the same state, delivered to onState:
//   { days, event, lagging, rel (Float64Array, km, three numbers a body in
//     the order of `ids`), shapes (array of [id, orbit] or null) }
// Each request works for at most budgetMs, then answers with where it got.
import { createGravitySim } from './orbits.js'

// The gravity itself, the same in the worker and here.
export function runGravity(bodies, ids, { budgetMs = 30, chunk = 2000, ...options } = {}) {
  const sim = createGravitySim(bodies, { ...options, maxSteps: chunk })
  function state(reached, wantShapes) {
    const rel = new Float64Array(ids.length * 3)
    ids.forEach((id, i) => sim.relativeInto(id, rel, i * 3))
    return {
      days: sim.days,
      event: sim.event,
      lagging: !reached && !sim.event,
      rel,
      shapes: wantShapes ? ids.map((id) => [id, sim.orbitNow(id)]).filter(([, o]) => o) : null,
    }
  }
  return {
    advance(target, wantShapes) {
      const start = Date.now()
      let reached = sim.advanceTo(target)
      while (!reached && !sim.event && Date.now() - start < budgetMs) reached = sim.advanceTo(target)
      return state(reached, wantShapes)
    },
    reset(wantShapes) {
      sim.reset()
      return state(true, wantShapes)
    },
  }
}

// One gravity run, for these bodies. Requests are answered in order; while
// one is being worked on, a newer advance replaces any waiting one.
export function createGravityEngine(bodies, ids, options, onState) {
  // Plain copies: the page's bodies may be reactive, which can't be posted.
  const plain = JSON.parse(JSON.stringify(bodies))
  if (typeof Worker === 'undefined') {
    const run = runGravity(plain, ids, options)
    return {
      advance: (target, shapes) => onState(run.advance(target, shapes)),
      reset: (shapes) => onState(run.reset(shapes)),
      dispose() {},
    }
  }
  const worker = new Worker(new URL('./gravity.worker.js', import.meta.url), { type: 'module' })
  let busy = false
  let waiting = null // the next request, sent when the worker answers
  const send = (msg) => {
    busy = true
    worker.postMessage(msg)
  }
  worker.onmessage = (e) => {
    busy = false
    if (waiting) {
      const next = waiting
      waiting = null
      send(next)
    }
    onState(e.data)
  }
  worker.postMessage({ type: 'init', bodies: plain, ids, options }) // answered by nothing
  const request = (msg) => {
    // A reset must not be lost; an advance can replace an older advance.
    if (!busy) send(msg)
    else if (!waiting || waiting.type !== 'reset' || msg.type === 'reset') waiting = msg
    else waiting = { ...msg, type: 'reset-then-advance' }
  }
  return {
    advance: (target, shapes) => request({ type: 'advance', target, shapes }),
    reset: (shapes) => request({ type: 'reset', shapes }),
    dispose: () => worker.terminate(),
  }
}
