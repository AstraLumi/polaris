// Works out the planet's climate (climateModel.js) off the page's thread:
// it takes a second or two on a big map. planetClimate.js talks to it.
import { simulateClimate } from './climateModel.js'

self.onmessage = (e) => {
  const { id, input } = e.data
  try {
    const result = simulateClimate(input)
    const c = result.coarse
    const transfer = [c.landFrac, c.temp, c.precip, c.humidity, c.pressure, c.wind, c.current, c.sea, result.interp.idx, result.interp.w].map((a) => a.buffer)
    self.postMessage({ id, result }, transfer)
  } catch (err) {
    self.postMessage({ id, error: String(err?.message || err) })
  }
}
