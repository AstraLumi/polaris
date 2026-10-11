// Lays out a family tree (views/FamilyTree.vue) in generation rows: parents
// above their children, partners side by side, siblings next to each other.
// No dependencies: node frontend/tests/familyLayout.test.mjs
//
// people: [{ id, ... }], links: [{ from, to, kind, until }] where kind is
// parent (from = the parent), sibling, spouse or partner. root: the person
// the tree is drawn around (generation 0 before normalising).

export const CARD_W = 168
export const CARD_H = 64
export const PARTNER_GAP = 30 // between two partners (the line joining them)
export const UNIT_GAP = 40 // between one family unit and the next
export const ROW_H = 150

export function layoutFamily(people, links, root) {
  const has = new Set(people.map((p) => p.id))
  const order = new Map(people.map((p, i) => [p.id, i]))
  const make = () => new Map(people.map((p) => [p.id, new Set()]))
  const parents = make()
  const children = make()
  const partners = make()
  const siblings = make()
  const couples = []
  for (const l of links) {
    if (!has.has(l.from) || !has.has(l.to) || l.from === l.to) continue
    if (l.kind === 'parent') {
      parents.get(l.to).add(l.from)
      children.get(l.from).add(l.to)
    } else if (l.kind === 'sibling') {
      siblings.get(l.from).add(l.to)
      siblings.get(l.to).add(l.from)
    } else if (l.kind === 'spouse' || l.kind === 'partner') {
      partners.get(l.from).add(l.to)
      partners.get(l.to).add(l.from)
      couples.push({ a: l.from, b: l.to, ended: !!l.until })
    }
  }

  // ---- generations: walk out from the root ----
  const gen = new Map()
  const walk = (start, g) => {
    gen.set(start, g)
    const queue = [start]
    const set = (id, v) => {
      if (!gen.has(id)) {
        gen.set(id, v)
        queue.push(id)
      }
    }
    while (queue.length) {
      const id = queue.shift()
      const g0 = gen.get(id)
      for (const p of parents.get(id)) set(p, g0 - 1)
      for (const c of children.get(id)) set(c, g0 + 1)
      for (const s of partners.get(id)) set(s, g0)
      for (const s of siblings.get(id)) set(s, g0)
    }
  }
  if (has.has(root)) walk(root, 0)
  for (const p of people) if (!gen.has(p.id)) walk(p.id, 0)
  const minGen = Math.min(...gen.values())
  for (const [id, g] of gen) gen.set(id, g - minGen)
  const maxGen = Math.max(0, ...gen.values())

  // ---- units: partners in the same generation stay together ----
  const unitOf = new Map()
  const rows = Array.from({ length: maxGen + 1 }, () => [])
  for (const p of people) {
    if (unitOf.has(p.id)) continue
    const g = gen.get(p.id)
    // Everyone joined to p through partners of the same generation.
    const group = []
    const stack = [p.id]
    const inGroup = new Set([p.id])
    while (stack.length) {
      const id = stack.pop()
      group.push(id)
      for (const q of partners.get(id)) {
        if (!inGroup.has(q) && gen.get(q) === g) {
          inGroup.add(q)
          stack.push(q)
        }
      }
    }
    // As a chain, so each partner line joins two neighbours: start at an end.
    const degree = (id) => [...partners.get(id)].filter((q) => inGroup.has(q)).length
    group.sort((a, b) => degree(a) - degree(b) || order.get(a) - order.get(b))
    const members = []
    const placed = new Set()
    let cur = group[0]
    while (cur != null) {
      members.push(cur)
      placed.add(cur)
      const next = [...partners.get(cur)].filter((q) => inGroup.has(q) && !placed.has(q)).sort((a, b) => order.get(a) - order.get(b))
      cur = next.length ? next[0] : group.find((q) => !placed.has(q))
    }
    const unit = { members, gen: g, left: 0, w: members.length * CARD_W + (members.length - 1) * PARTNER_GAP }
    for (const m of members) unitOf.set(m, unit)
    rows[g].push(unit)
  }

  const centerOf = (id) => {
    const u = unitOf.get(id)
    return u.left + u.members.indexOf(id) * (CARD_W + PARTNER_GAP) + CARD_W / 2
  }
  const unitCenter = (u) => u.left + u.w / 2
  // The average centre of a unit's parents (or children), or null.
  const bary = (u, rel) => {
    const xs = u.members.flatMap((m) => [...rel.get(m)].filter((q) => unitOf.get(q) !== u).map(centerOf))
    return xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null
  }
  const pack = (row) => {
    let x = 0
    for (const u of row) {
      u.left = x
      x += u.w + UNIT_GAP
    }
  }
  // Brothers and sisters sort next to each other: same parents, same key.
  const familyKey = (u) =>
    u.members
      .map((m) => [...parents.get(m)].sort((a, b) => a - b).join(','))
      .filter(Boolean)
      .sort()[0] || ''
  const firstOrder = (u) => Math.min(...u.members.map((m) => order.get(m)))

  for (const row of rows) {
    row.sort((a, b) => familyKey(a).localeCompare(familyKey(b)) || firstOrder(a) - firstOrder(b))
    pack(row)
  }

  // ---- order within each row: follow the parents down, the children up ----
  // A unit with no parents (or children) to follow sorts just after a
  // sibling's unit, and is then placed snug beside it.
  const siblingUnits = (u) =>
    new Set(u.members.flatMap((m) => [...siblings.get(m)].map((s) => unitOf.get(s))).filter((s) => s !== u && s.gen === u.gen))
  const siblingUnit = (u) => siblingUnits(u).values().next().value || null
  const sortRow = (row, rel) => {
    // Anchored units sort by their parents (or children); the rest follow a
    // sibling that is anchored, directly or through another sibling.
    const key = new Map()
    for (const u of row) {
      const b = bary(u, rel)
      if (b != null) key.set(u, b)
    }
    for (let changed = true; changed; ) {
      changed = false
      for (const u of row) {
        if (key.has(u)) continue
        const s = [...siblingUnits(u)].find((x) => key.has(x))
        if (s) {
          key.set(u, key.get(s) + 0.01)
          changed = true
        }
      }
    }
    for (const u of row) if (!key.has(u)) key.set(u, unitCenter(u))
    row.sort((a, b) => key.get(a) - key.get(b) || familyKey(a).localeCompare(familyKey(b)) || firstOrder(a) - firstOrder(b))
    pack(row)
  }
  for (let i = 0; i < 4; i++) {
    for (let g = 1; g <= maxGen; g++) sortRow(rows[g], parents)
    for (let g = maxGen - 1; g >= 0; g--) sortRow(rows[g], children)
  }

  // ---- positions: centre parents over children and children under parents ----
  const place = (row, rel) => {
    const want = row.map((u) => bary(u, rel))
    let right = -Infinity
    for (let i = 0; i < row.length; i++) {
      const u = row[i]
      // Nothing to follow: beside a sibling, or else where it already is.
      const desired = want[i] != null ? want[i] - u.w / 2 : i > 0 && siblingUnit(u) ? -Infinity : u.left
      u.left = Math.max(desired, right + UNIT_GAP)
      right = u.left + u.w
    }
    // Shift the row as a whole so it sits, on average, where it wants to be.
    let sum = 0
    let n = 0
    row.forEach((u, i) => {
      if (want[i] != null) {
        sum += want[i] - unitCenter(u)
        n++
      }
    })
    if (n) for (const u of row) u.left += sum / n
  }
  for (let i = 0; i < 6; i++) {
    for (let g = maxGen - 1; g >= 0; g--) place(rows[g], children)
    for (let g = 1; g <= maxGen; g++) place(rows[g], parents)
  }

  // ---- output ----
  let minX = Infinity
  let maxX = -Infinity
  for (const row of rows) {
    for (const u of row) {
      minX = Math.min(minX, u.left)
      maxX = Math.max(maxX, u.left + u.w)
    }
  }
  const pos = new Map()
  for (const p of people) {
    const u = unitOf.get(p.id)
    pos.set(p.id, {
      x: u.left - minX + u.members.indexOf(p.id) * (CARD_W + PARTNER_GAP),
      y: u.gen * ROW_H,
      gen: u.gen,
    })
  }

  // Partner lines: side by side in one unit, or (across generations) apart.
  for (const c of couples) {
    const u = unitOf.get(c.a)
    const ia = u.members.indexOf(c.a)
    const ib = u.members.indexOf(c.b)
    c.adjacent = ib >= 0 && Math.abs(ia - ib) === 1
  }

  // Children grouped by their (known) parents.
  const families = new Map()
  for (const p of people) {
    const ps = [...parents.get(p.id)].sort((a, b) => a - b)
    if (!ps.length) continue
    const key = ps.join(',')
    if (!families.has(key)) families.set(key, { parents: ps, children: [] })
    families.get(key).children.push(p.id)
  }
  for (const f of families.values()) f.children.sort((a, b) => pos.get(a).x - pos.get(b).x)

  // Siblings with no parent in the tree to hang from.
  const siblingPairs = []
  for (const [a, set] of siblings) {
    for (const b of set) {
      if (a >= b) continue
      const shared = [...parents.get(a)].some((p) => parents.get(b).has(p))
      if (!shared) siblingPairs.push({ a, b })
    }
  }

  return {
    pos,
    couples,
    families: [...families.values()],
    siblingPairs,
    width: maxX - minX,
    height: maxGen * ROW_H + CARD_H,
  }
}
