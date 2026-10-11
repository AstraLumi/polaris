// Runs inside a Polaris page (same origin): builds the "The Ashen Crown" demo
// story used for the README screenshots. Returns { story, ids }.
(async () => {
  const j = async (r) => {
    if (!r.ok) throw new Error(r.url + ' ' + r.status + ' ' + (await r.text()))
    return r.json()
  }
  const form = (o) => {
    const f = new FormData()
    for (const [k, v] of Object.entries(o)) if (v !== undefined && v !== null) f.append(k, v)
    return f
  }
  const story = await j(await fetch('/api/stories', { method: 'POST', body: form({ name: 'The Ashen Crown' }) }))
  const S = `/api/s/${story.id}`
  const post = (p, body) => fetch(S + p, { method: 'POST', body }).then(j)
  const put = (p, body) => fetch(S + p, { method: 'PUT', body }).then(j)
  const json = (m, p, o) => fetch(S + p, { method: m, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(o) }).then(j)

  // ---- pictures, drawn on a canvas ----
  const hsl = (h, s, l) => `hsl(${h} ${s}% ${l}%)`
  async function png(draw, w = 320, h = 320) {
    const c = document.createElement('canvas')
    c.width = w
    c.height = h
    draw(c.getContext('2d'), w, h)
    const blob = await new Promise((r) => c.toBlob(r, 'image/png'))
    return new File([blob], 'pic.png', { type: 'image/png' })
  }
  // A simple bust: coloured background, shoulders, head, hair.
  const portrait = (hue, skin, hair) =>
    png((x, w, h) => {
      const g = x.createLinearGradient(0, 0, 0, h)
      g.addColorStop(0, hsl(hue, 45, 42))
      g.addColorStop(1, hsl(hue + 30, 50, 18))
      x.fillStyle = g
      x.fillRect(0, 0, w, h)
      x.fillStyle = hsl(hue + 180, 25, 30)
      x.beginPath()
      x.ellipse(w / 2, h * 1.02, w * 0.42, h * 0.32, 0, 0, Math.PI * 2)
      x.fill()
      x.fillStyle = skin
      x.beginPath()
      x.ellipse(w / 2, h * 0.47, w * 0.17, h * 0.21, 0, 0, Math.PI * 2)
      x.fill()
      x.fillStyle = hair
      x.beginPath()
      x.ellipse(w / 2, h * 0.36, w * 0.2, h * 0.14, 0, Math.PI, Math.PI * 2)
      x.fill()
      x.fillRect(w * 0.3, h * 0.35, w * 0.06, h * 0.22)
      x.fillRect(w * 0.64, h * 0.35, w * 0.06, h * 0.22)
    })
  // A landscape: sky, sun, two ridges.
  const landscape = (hue, sun = true) =>
    png(
      (x, w, h) => {
        const g = x.createLinearGradient(0, 0, 0, h)
        g.addColorStop(0, hsl(hue, 55, 70))
        g.addColorStop(1, hsl(hue + 20, 45, 35))
        x.fillStyle = g
        x.fillRect(0, 0, w, h)
        if (sun) {
          x.fillStyle = hsl(45, 90, 80)
          x.beginPath()
          x.arc(w * 0.72, h * 0.3, h * 0.12, 0, Math.PI * 2)
          x.fill()
        }
        for (const [lvl, l] of [[0.55, 30], [0.7, 18]]) {
          x.fillStyle = hsl(hue + 140, 25, l)
          x.beginPath()
          x.moveTo(0, h)
          for (let i = 0; i <= 12; i++) x.lineTo((w * i) / 12, h * lvl + Math.sin(i * 1.7 + lvl * 9) * h * 0.08)
          x.lineTo(w, h)
          x.fill()
        }
      },
      640,
      400,
    )

  // ---- calendar, chapters ----
  const vol = await json('POST', '/volumes', { title: 'Book One: Embers' })
  const chapters = []
  for (const title of ['The Burning Archive', 'Salt and Ash', 'The Adoption', 'Starfall', 'The Long Road North']) {
    chapters.push((await json('POST', '/chapters', { title, volume_id: vol.id })).id)
  }

  // ---- map ----
  const disc = (cq, cr, R, skip = () => false) => {
    const out = []
    for (let q = -R; q <= R; q++)
      for (let r = Math.max(-R, -q - R); r <= Math.min(R, -q + R); r++) if (!skip(q, r)) out.push([cq + q, cr + r])
    return out
  }
  const loc = (o) => json('POST', '/map/locations', o)
  const kingdoms = {}
  for (const [name, color, cq, cr, R, founded, desc] of [
    ['Aldoria', '#c75454', 0, 0, 4, '01-03-0412', 'A walled realm on the western coast, famed for its academies.'],
    ['Eastmarch', '#4f8fd0', 8, -3, 3, '15-06-0388', 'Hill country of herders and old border forts.'],
    ['Saltmere', '#d6a447', 3, 6, 3, '01-01-0501', 'Salt flats and harbour towns along the southern sea.'],
    ['Velmar', '#5fae7d', -6, 6, 3, '20-09-0455', 'Deep forests ruled from a city built into the trees.'],
  ]) {
    const l = await loc({ kind: 'major', name, color, founding_date: founded, description: desc })
    kingdoms[name] = l.id
    await json('POST', '/map/paint', { location_id: l.id, hexes: disc(cq, cr, R, (q, r) => (q * 7 + r * 3) % 11 === 0 && Math.abs(q) + Math.abs(r) > R) })
  }
  const mist = await loc({ kind: 'major', name: 'Mistwood', color: '', description: 'An old forest no kingdom has tamed.' })
  await json('POST', '/map/paint', { location_id: mist.id, hexes: disc(-3, 2, 1) })
  const places = {}
  for (const [name, q, r, city, capital] of [
    ['Aldor', 0, 0, true, true], ['Highwatch', 2, -3, true, false], ['Saltmere Port', 3, 7, true, true],
    ['Kestrel Ford', 7, -2, false, false], ['Eastgate', 8, -4, true, true], ['Velmar Hollow', -6, 6, true, true], ['Thornby', -2, 3, false, false],
  ]) {
    places[name] = (await loc({ kind: 'minor', name, q, r, is_city: city, is_capital: capital })).id
  }

  // ---- characters ----
  const people = {}
  const versions = {}
  async function person(o) {
    const c = await post('/characters', form({ name: o.name, level: String(o.level || 1) }))
    people[o.key] = c.id
    versions[o.key] = c.version_id
    await sheet(c.version_id, o)
    return c
  }
  async function sheet(vid, o) {
    const fd = form({
      name: o.name, nickname: o.nickname || '', level: String(o.level || 1), class: o.class || '', specialization: o.spec || '',
      race: o.race || '', gender: o.gender || '', age: o.age ?? '', blood_type: o.blood || '', birth_date: o.born || '',
      born_in_location_id: o.bornIn ? String(places[o.bornIn] || kingdoms[o.bornIn]) : '', nation_location_id: o.nation ? String(kingdoms[o.nation]) : '',
      status: o.status || 'alive', chapter_id: o.chapter ? String(chapters[o.chapter - 1]) : '', description: o.description || '', bio: o.bio || '',
      tags: JSON.stringify(o.tags || []),
      // Points in the default primary stats, in order.
      build: JSON.stringify(Object.fromEntries(['vit', 'def', 'res', 'str', 'dex', 'intel', 'wis', 'agl'].map((k, i) => [k, o.stats?.[i] || 0]))),
    })
    if (o.portrait) fd.append('picture', await portrait(...o.portrait))
    return put(`/versions/${vid}`, fd)
  }
  const skin = ['#e8c4a2', '#c99a74', '#8d5f3e', '#f1d6bd']
  await person({ key: 'aria', name: 'Aria', nickname: 'Ash', level: 12, class: 'Starcaller', spec: 'Spellslinger', race: 'Human', gender: 'Female', age: 16, blood: 'A+', born: '30-04-1591', bornIn: 'Thornby', nation: 'Aldoria', chapter: 1, tags: ['Protagonist'], stats: [2, 1, 1, 0, 2, 4, 2, 0], portrait: [10, skin[0], '#3a2418'], description: 'A soot-streaked archive apprentice with a burn scar across one hand.' })
  await person({ key: 'bram', name: 'Bram Vale', level: 30, class: 'Starcaller', race: 'Human', gender: 'Male', age: 52, blood: 'O-', born: '02-11-1555', bornIn: 'Aldor', nation: 'Aldoria', tags: ['Mentor'], stats: [4, 4, 4, 2, 2, 8, 6, 0], portrait: [220, skin[1], '#d9d9d9'] })
  await person({ key: 'mira', name: 'Mira Vale', level: 24, class: 'Warden', race: 'Human', gender: 'Female', age: 49, blood: 'B+', born: '17-02-1558', bornIn: 'Highwatch', nation: 'Aldoria', stats: [6, 6, 2, 6, 2, 0, 2, 0], portrait: [190, skin[3], '#7a3b1e'] })
  await person({ key: 'cael', name: 'Cael Vale', level: 14, class: 'Warden', race: 'Human', gender: 'Male', age: 21, born: '09-08-1586', bornIn: 'Aldor', nation: 'Aldoria', stats: [4, 4, 0, 4, 2, 0, 0, 0], portrait: [200, skin[1], '#2a1a12'] })
  await person({ key: 'lira', name: 'Lira Vale', level: 9, class: 'Starcaller', race: 'Human', gender: 'Female', age: 18, born: '01-05-1589', bornIn: 'Aldor', nation: 'Aldoria', stats: [1, 1, 1, 0, 2, 2, 2, 0], portrait: [280, skin[3], '#2a1a12'] })
  await person({ key: 'oren', name: 'Oren Ashford', level: 20, class: 'Blade', race: 'Human', gender: 'Male', age: 47, status: 'dead', born: '21-07-1560', bornIn: 'Thornby', nation: 'Aldoria', stats: [4, 4, 0, 8, 4, 0, 0, 0], portrait: [20, skin[2], '#141414'] })
  await person({ key: 'sela', name: 'Sela Ashford', level: 11, class: 'Healer', race: 'Human', gender: 'Female', age: 44, status: 'missing', born: '05-12-1563', bornIn: 'Thornby', nation: 'Aldoria', stats: [2, 2, 2, 0, 0, 2, 3, 0], portrait: [330, skin[0], '#9a5a2a'] })
  await person({ key: 'thane', name: 'Thane Morrow', level: 26, class: 'Blade', race: 'Elf', gender: 'Male', age: 230, born: '10-10-1377', bornIn: 'Velmar Hollow', nation: 'Velmar', tags: ['Antagonist'], stats: [4, 4, 4, 6, 6, 0, 2, 0], portrait: [140, skin[3], '#e8e0c8'] })
  await person({ key: 'nyx', name: 'Nyx', level: 15, class: 'Shade', race: 'Elf', gender: 'Female', age: 120, bornIn: 'Velmar Hollow', nation: 'Velmar', tags: ['Rival'], stats: [2, 2, 2, 2, 6, 1, 0, 0], portrait: [260, skin[2], '#1a1a2e'] })
  await person({ key: 'dov', name: 'Dov Harker', level: 8, class: 'Blade', race: 'Human', gender: 'Male', age: 17, bornIn: 'Saltmere Port', nation: 'Saltmere', tags: ['Ally'], stats: [2, 2, 0, 2, 2, 0, 0, 0], portrait: [40, skin[1], '#5a3418'] })

  // Aria later: adopted into the Vale family and renamed.
  const later = await json('POST', `/characters/${people.aria}/versions`, { version_date: '', version_reference: '', clone_from_version_id: versions.aria })
  await sheet(later.id, { name: 'Aria Vale', nickname: 'Ash', level: 18, class: 'Starcaller', spec: 'Spellslinger', race: 'Human', gender: 'Female', age: 17, blood: 'A+', born: '30-04-1591', bornIn: 'Thornby', nation: 'Aldoria', chapter: 3, tags: ['Protagonist'], stats: [2, 2, 2, 0, 3, 5, 3, 1], portrait: [10, skin[0], '#3a2418'], description: 'A soot-streaked archive apprentice with a burn scar across one hand.' })
  await post(`/versions/${later.id}/set-current`, null)

  // ---- relations ----
  const rel = (a, b, kind, o = {}) => json('POST', '/relations', { from_id: people[a], to_id: people[b], kind, ...o })
  await rel('oren', 'sela', 'spouse')
  await rel('oren', 'aria', 'parent')
  await rel('sela', 'aria', 'parent')
  await rel('bram', 'mira', 'spouse')
  for (const c of ['cael', 'lira']) {
    await rel('bram', c, 'parent')
    await rel('mira', c, 'parent')
  }
  await rel('bram', 'aria', 'mentor', { until_chapter_id: chapters[2] })
  await rel('bram', 'aria', 'parent', { since_chapter_id: chapters[2] })
  await rel('mira', 'aria', 'parent', { since_chapter_id: chapters[2] })
  await rel('aria', 'dov', 'friend')
  await rel('aria', 'nyx', 'rival')
  await rel('thane', 'nyx', 'mentor')
  await rel('thane', 'bram', 'enemy')
  await rel('cael', 'dov', 'ally')

  // ---- factions ----
  const fac = (o) => post('/factions', form(o))
  const academy = await fac({ name: 'Starfall Academy', description: 'The oldest school of star magic in the west.', color: '#8a7fd6', hq_location_id: String(places.Aldor), founding_date: '01-09-0430', members: JSON.stringify([{ character_id: people.bram, role: 'Archmage' }, { character_id: people.aria, role: 'Student' }, { character_id: people.lira, role: 'Student' }]) })
  await fac({ name: 'Ember Guard', description: 'The archive wardens of Aldor.', hq_location_id: String(places.Aldor), parent_id: String(academy.id), members: JSON.stringify([{ character_id: people.mira, role: 'Captain' }, { character_id: people.cael, role: 'Warden' }]) })
  await fac({ name: 'The Hollow Court', description: 'Velmar\'s hidden council.', hq_location_id: String(places['Velmar Hollow']), members: JSON.stringify([{ character_id: people.thane, role: 'Speaker' }, { character_id: people.nyx, role: 'Blade' }]) })

  // ---- events ----
  const ev = async (name, date, place, who, tags, chapter, desc = '') =>
    post('/events', form({ name, event_date: date, location_id: place ? String(places[place] || kingdoms[place]) : '', chapter_id: chapter ? String(chapters[chapter - 1]) : '', tags: JSON.stringify(tags), character_ids: JSON.stringify(who.map((k) => people[k])), description: desc }))
  await ev('Founding of the Starfall Academy', '01-09-0430', 'Aldor', [], ['academy'])
  await ev('The Saltmere Treaty', '14-05-1402', 'Saltmere Port', [], ['war'])
  await ev('The Border War begins', '03-03-1570', 'Eastgate', ['oren', 'bram'], ['war'])
  await ev('The Border War ends', '22-10-1574', 'Kestrel Ford', ['oren', 'bram', 'thane'], ['war'])
  await ev('The Archive burns', '11-01-1607', 'Aldor', ['aria', 'oren', 'sela', 'thane'], ['fire'], 1, 'The night the Ashford family was lost.')
  await ev('Aria joins the Academy', '02-02-1607', 'Aldor', ['aria', 'bram', 'lira'], ['academy'], 2)
  await ev('Aria is adopted by the Vales', '20-06-1607', 'Aldor', ['aria', 'bram', 'mira', 'cael', 'lira'], ['family'], 3)
  await ev('Starfall over Highwatch', '30-09-1607', 'Highwatch', ['aria', 'nyx', 'dov'], ['fire', 'academy'], 4)
  await ev('The road north', '05-12-1607', 'Kestrel Ford', ['aria', 'dov', 'cael'], [], 5)

  // ---- wiki ----
  const wput = (type, id, o) => json('PUT', `/wiki/${type}/${id}`, o)
  await wput('character', people.aria, {
    summary: '**Aria Vale**, born Aria Ashford, is an apprentice of the [[Starfall Academy]] and the last of the Ashfords of [[Thornby]]. After [[The Archive burns|the archive fire]] she was taken in by [[Bram Vale]].',
    fields: {
      appearance: 'Short, ash-dark hair and a burn scar across her right hand. She wears an Academy coat two sizes too large.',
      personality: 'Quiet until cornered, then stubborn. Keeps lists of everything.',
      background: 'Raised above the archive where her parents worked, she survived the fire that took them. [[Bram Vale|Bram]] was her teacher before he was her father.',
    },
    sections: [{ title: 'Magic', body: 'Aria calls starlight the way others strike flint: badly at first, then all at once.\n\n- Reads star charts by touch\n- Can hold a ward for a full night' }],
    infobox: [{ label: 'Weapon', value: 'A broken archive key' }],
  })
  await wput('location', kingdoms.Aldoria, {
    summary: 'The **Kingdom of Aldoria** is a realm on the western coast, ruled from [[Aldor]]. It is famed for the [[Starfall Academy]] and for its long rivalry with [[Eastmarch]].',
    fields: { geography: 'Rolling chalk hills meet a rocky coast. The northern bay is guarded by [[Highwatch]].', culture: 'Scholarship is prized above lineage. Every child is taught to read star charts.' },
    infobox: [{ label: 'Motto', value: '*Under every star*' }],
  })
  const gods = await post('/lore', form({ name: 'The Old Gods', picture: await landscape(260, false) }))
  await wput('lore', gods.id, {
    summary: 'The gods of the age before the [[Starfall Academy]], still worshipped quietly in [[Mistwood]] and [[Velmar]].',
    infobox: [{ label: 'Last temple razed', value: '1402', kind: 'date' }],
  })
  const starfall = await post('/lore', form({ name: 'Starfall', picture: await landscape(230) }))
  await wput('lore', starfall.id, {
    summary: 'A rain of falling stars that marks the turning points of Aldorian history; the Academy reads each one as a warning.',
    infobox: [{ label: 'Last seen', value: '30-09-1607', kind: 'date' }],
  })
  const gallery = new FormData()
  for (const h of [10, 200, 30]) gallery.append('pictures', await landscape(h))
  await post(`/wiki/location/${kingdoms.Aldoria}/gallery`, gallery)

  return { story: story.id, ids: { people, kingdoms, places, chapters, versions, later: later.id } }
})()
