import assert from 'node:assert/strict'
import { TILE, cardRect, linkLine, resizeRect, freeSpot, boardBounds, fitView, inkOn, MIN_W, MAX_H } from '../src/drafts/board.js'

// A closed card is a tile; an open one takes its own size.
const card = { x: 10, y: 20, w: 400, h: 300 }
assert.deepEqual(cardRect(card), { x: 10, y: 20, w: TILE.w, h: TILE.h })
assert.deepEqual(cardRect(card, true), { x: 10, y: 20, w: 400, h: 300 })

// A line runs edge to edge, its middle between them.
const a = { x: 0, y: 0, w: 100, h: 50 }
const b = { x: 300, y: 0, w: 100, h: 50 }
assert.deepEqual(linkLine(a, b), { x1: 100, y1: 25, x2: 300, y2: 25, mx: 200, my: 25 })
const below = linkLine(a, { x: 0, y: 200, w: 100, h: 50 })
assert.equal(below.y1, 50)
assert.equal(below.y2, 200)
// Overlapping cards: nothing to draw.
assert.equal(linkLine(a, { x: 10, y: 5, w: 100, h: 50 }), null)

// Resizing: the far side stays put, the limits hold.
assert.deepEqual(resizeRect(card, 'se', 50, -20), { x: 10, y: 20, w: 450, h: 280 })
assert.deepEqual(resizeRect(card, 'w', 30, 0), { x: 40, y: 20, w: 370, h: 300 })
assert.deepEqual(resizeRect(card, 'w', 1000, 0), { x: 10 + 400 - MIN_W, y: 20, w: MIN_W, h: 300 })
assert.equal(resizeRect(card, 'n', 0, -5000).h, MAX_H)
assert.equal(resizeRect(card, 'n', 0, -5000).y, 20 + 300 - MAX_H)

// A new tile never lands on another.
const tiles = [{ x: -90, y: -32 }]
const spot = freeSpot(tiles, 0, 0)
assert.ok(spot.y >= -32 + TILE.h + 12 || spot.x >= -90 + TILE.w + 12, JSON.stringify(spot))
assert.deepEqual(freeSpot([], 0, 0), { x: -90, y: -32 })

// Showing everything: the box's middle lands in the viewport's middle.
const box = boardBounds([a, b])
assert.deepEqual(box, { minX: 0, minY: 0, maxX: 400, maxY: 50 })
const v = fitView(box, 800, 600)
assert.equal(v.k, 1.25)
assert.equal(v.x + 200 * v.k, 400)
assert.equal(v.y + 25 * v.k, 300)
assert.deepEqual(fitView(null, 800, 600), { x: 400, y: 300, k: 1 })

// Text on a category's colour.
assert.equal(inkOn('#ffffff'), '#14161c')
assert.equal(inkOn('#1f4e79'), '#ffffff')
assert.equal(inkOn('nope'), '')

console.log('draftBoard: ok')
