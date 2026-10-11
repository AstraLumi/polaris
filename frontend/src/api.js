import { t, tr } from './i18n'
import { STORY_API } from './stories'

// Everything here belongs to the open story (see stories.js).
const BASE = STORY_API

// Errors shown to the user. The text is either one of this file's English
// fallbacks or a message the server sent (also English); either way it goes
// through t(), so anything with a translation shows in the chosen language.
function apiError(message) {
  return new Error(t(String(message).trim()))
}

export async function fetchCharacters() {
  const res = await fetch(`${BASE}/characters`)
  if (!res.ok) throw apiError('failed to load characters')
  return res.json()
}

export async function createCharacter({ name, nickname, level, className, subclassName, specializationName, versionDate, versionReference, picture }) {
  const formData = new FormData()
  formData.append('name', name)
  formData.append('nickname', nickname || '')
  formData.append('level', String(level || 1))
  formData.append('class', className || '')
  formData.append('subclass', subclassName || '')
  formData.append('specialization', specializationName || '')
  formData.append('version_date', versionDate || '')
  formData.append('version_reference', versionReference || '')
  if (picture) formData.append('picture', picture)

  const res = await fetch(`${BASE}/characters`, { method: 'POST', body: formData })
  if (!res.ok) throw apiError('failed to create character')
  return res.json()
}

export async function deleteCharacters(ids) {
  const res = await fetch(`${BASE}/characters`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  })
  if (!res.ok) throw apiError('failed to delete characters')
  return res.json()
}

export async function fetchCurrentVersion(characterId) {
  const res = await fetch(`${BASE}/characters/${characterId}/current`)
  if (!res.ok) throw apiError('failed to load current version')
  return res.json()
}

export async function fetchVersions(characterId) {
  const res = await fetch(`${BASE}/characters/${characterId}/versions`)
  if (!res.ok) throw apiError('failed to load versions')
  return res.json()
}

export async function createVersion(characterId, { versionDate, versionReference, cloneFromVersionId, chapterId }) {
  const res = await fetch(`${BASE}/characters/${characterId}/versions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      version_date: versionDate || '',
      version_reference: versionReference || '',
      clone_from_version_id: cloneFromVersionId ?? null,
      chapter_id: chapterId ?? null,
    }),
  })
  if (!res.ok) throw apiError('failed to create version')
  return res.json()
}

export async function fetchVersion(versionId) {
  const res = await fetch(`${BASE}/versions/${versionId}`)
  if (!res.ok) throw apiError('failed to load version')
  return res.json()
}

export async function updateVersion(versionId, formData) {
  const res = await fetch(`${BASE}/versions/${versionId}`, { method: 'PUT', body: formData })
  if (!res.ok) {
    const message = await res.text()
    throw apiError(message || 'failed to save version')
  }
  return res.json()
}

export async function deleteVersion(versionId) {
  const res = await fetch(`${BASE}/versions/${versionId}`, { method: 'DELETE' })
  if (!res.ok) {
    const message = await res.text()
    throw apiError(message || 'failed to delete version')
  }
  return res.json()
}

export async function setCurrentVersion(versionId) {
  const res = await fetch(`${BASE}/versions/${versionId}/set-current`, { method: 'POST' })
  if (!res.ok) throw apiError('failed to set current version')
  return res.json()
}

// Mirrors backend/stats.go's target_stat keys, grouped for display in the
// asset modifier form. Every group here is a flat-modifier target — the
// same shape used across every asset type, not just Races.
export const MODIFIER_GROUPS = [
  {
    label: tr('Primary Stats'),
    keys: [
      ['vit', 'VIT'], ['def', 'DEF'], ['res', 'RES'], ['str', 'STR'],
      ['dex', 'DEX'], ['intel', 'INT'], ['wis', 'WIS'], ['agl', 'AGL'],
    ],
  },
  {
    label: tr('Base Stats'),
    keys: [
      ['hp', 'HP'], ['mp', 'MP'], ['attack', tr('Attack')], ['potency', tr('Potency')],
      ['defense', tr('Defense')], ['resistance', tr('Resistance')], ['speed', tr('Speed')],
    ],
  },
  {
    label: tr('Special Stats'),
    keys: [
      ['luck', tr('Luck')], ['carry_limit', tr('Carry Limit')], ['fall_damage_threshold', tr('Fall Damage Threshold')],
      ['sanity', tr('Sanity')], ['heat_threshold', tr('Heat Threshold')], ['cold_threshold', tr('Cold Threshold')],
    ],
  },
  {
    label: tr('Special Defenses'),
    keys: [
      ['resist_water', tr('Water')], ['resist_fire', tr('Fire')], ['resist_wind', tr('Wind')], ['resist_earth', tr('Earth')],
      ['resist_ice', tr('Ice')], ['resist_thunder', tr('Thunder')], ['resist_impact', tr('Impact')],
    ],
  },
  {
    label: tr('Lifeskill'),
    keys: [
      ['cooking', tr('Cooking')], ['crafting', tr('Crafting')], ['alchemy', tr('Alchemy')], ['hunting', tr('Hunting')],
      ['gathering', tr('Gathering')], ['farming', tr('Farming')], ['blessing', tr('Blessing')],
      ['enchanting', tr('Enchanting')], ['smithing', tr('Smithing')],
    ],
  },
]

// Generic CRUD for asset types that are just "a name plus modifiers"
// (Classes, Races, Body Types) — same shape, different endpoint.
function simpleAssetApi(basePath) {
  return {
    async fetch(id) {
      const res = await fetch(`${BASE}/${basePath}/${id}`)
      if (!res.ok) throw apiError(`failed to load ${basePath}`)
      return res.json()
    },
    async create(name, modifiers) {
      const res = await fetch(`${BASE}/${basePath}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, modifiers }),
      })
      if (!res.ok) throw apiError(await res.text() || `failed to create ${basePath}`)
      return res.json()
    },
    async update(id, name, modifiers) {
      const res = await fetch(`${BASE}/${basePath}/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, modifiers }),
      })
      if (!res.ok) throw apiError(await res.text() || `failed to update ${basePath}`)
      return res.json()
    },
    async delete(id) {
      const res = await fetch(`${BASE}/${basePath}/${id}`, { method: 'DELETE' })
      if (!res.ok) throw apiError(`failed to delete ${basePath}`)
      return res.json()
    },
  }
}

export const classesApi = {
  ...simpleAssetApi('classes'),
  // Class icons are uploaded after the class itself is saved.
  async setIcon(id, file) {
    const fd = new FormData()
    fd.append('icon', file)
    const res = await fetch(`${BASE}/classes/${id}/icon`, { method: 'PUT', body: fd })
    if (!res.ok) throw apiError((await res.text()).trim() || 'failed to save the icon')
    return res.json()
  },
  async clearIcon(id) {
    const res = await fetch(`${BASE}/classes/${id}/icon`, { method: 'DELETE' })
    if (!res.ok) throw apiError((await res.text()).trim() || 'failed to remove the icon')
    return res.json()
  },
}
export const racesApi = simpleAssetApi('races')
export const bodyTypesApi = simpleAssetApi('body-types')

export const specializationsApi = {
  async fetch(id) {
    const res = await fetch(`${BASE}/specializations/${id}`)
    if (!res.ok) throw apiError('failed to load specialization')
    return res.json()
  },
  async create(name, classId, modifiers) {
    const res = await fetch(`${BASE}/specializations`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, class_id: classId, modifiers }),
    })
    if (!res.ok) throw apiError(await res.text() || 'failed to create specialization')
    return res.json()
  },
  async update(id, name, classId, modifiers) {
    const res = await fetch(`${BASE}/specializations/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, class_id: classId, modifiers }),
    })
    if (!res.ok) throw apiError(await res.text() || 'failed to update specialization')
    return res.json()
  },
  async delete(id) {
    const res = await fetch(`${BASE}/specializations/${id}`, { method: 'DELETE' })
    if (!res.ok) throw apiError('failed to delete specialization')
    return res.json()
  },
}

export const subclassesApi = {
  async fetch(id) {
    const res = await fetch(`${BASE}/subclasses/${id}`)
    if (!res.ok) throw apiError('failed to load subclass')
    return res.json()
  },
  async create(name, classIds, modifiers) {
    const res = await fetch(`${BASE}/subclasses`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, class_ids: classIds, modifiers }),
    })
    if (!res.ok) throw apiError(await res.text() || 'failed to create subclass')
    return res.json()
  },
  async update(id, name, classIds, modifiers) {
    const res = await fetch(`${BASE}/subclasses/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, class_ids: classIds, modifiers }),
    })
    if (!res.ok) throw apiError(await res.text() || 'failed to update subclass')
    return res.json()
  },
  async delete(id) {
    const res = await fetch(`${BASE}/subclasses/${id}`, { method: 'DELETE' })
    if (!res.ok) throw apiError('failed to delete subclass')
    return res.json()
  },
}

// Computes ComputedStats from whatever's currently in the edit form
// (including unsaved changes), without touching the database or requiring
// the version to already exist. This is what powers the Edit page's live
// preview — it calls the real backend formulas (including modifiers)
// instead of duplicating them in JS.
export async function fetchComputePreview(payload) {
  const res = await fetch(`${BASE}/compute-preview`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw apiError('failed to compute preview')
  return res.json()
}
// Slim list of every Map location, for the Born in / Nation pickers.
export async function fetchLocationOptions() {
  const res = await fetch(`${BASE}/location-options`)
  if (!res.ok) return []
  return res.json()
}

export async function fetchSettings() {
  const res = await fetch(`${BASE}/settings`)
  if (!res.ok) throw apiError('failed to load settings')
  return res.json()
}

export async function saveSettings(payload) {
  const res = await fetch(`${BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw apiError((await res.text()).trim() || 'failed to save settings')
  return res.json()
}

export async function fetchHome() {
  const res = await fetch(`${BASE}/home`)
  if (!res.ok) throw apiError('failed to load home')
  return res.json()
}

export async function fetchCatalogs() {
  const [classes, subclasses, specializations, races, bodyTypes, locations] = await Promise.all([
    fetch(`${BASE}/classes`).then((r) => (r.ok ? r.json() : [])),
    fetch(`${BASE}/subclasses`).then((r) => (r.ok ? r.json() : [])),
    fetch(`${BASE}/specializations`).then((r) => (r.ok ? r.json() : [])),
    fetch(`${BASE}/races`).then((r) => (r.ok ? r.json() : [])),
    fetch(`${BASE}/body-types`).then((r) => (r.ok ? r.json() : [])),
    fetch(`${BASE}/locations`).then((r) => (r.ok ? r.json() : [])),
  ])
  return { classes, subclasses, specializations, races, bodyTypes, locations }
}

export const spellsApi = {
  async list() {
    const res = await fetch(`${BASE}/spells`)
    if (!res.ok) throw apiError('failed to load spells')
    return res.json()
  },
  async create(formData) {
    const res = await fetch(`${BASE}/spells`, { method: 'POST', body: formData })
    if (!res.ok) throw apiError((await res.text()) || 'failed to create spell')
    return res.json()
  },
  async update(id, formData) {
    const res = await fetch(`${BASE}/spells/${id}`, { method: 'PUT', body: formData })
    if (!res.ok) throw apiError((await res.text()) || 'failed to update spell')
    return res.json()
  },
  async delete(id) {
    const res = await fetch(`${BASE}/spells/${id}`, { method: 'DELETE' })
    if (!res.ok) throw apiError('failed to delete spell')
    return res.json()
  },
}

// `source` is "type:id" (e.g. "class:3") or '' for none.
// iconChoice: '' (leave the icon alone), 'none', or 'builtin:<id>'. An
// uploaded file wins over it.
export function buildSpellFormData({ name, level, source, mpCost, hpCost, origin, description }, iconFile, iconChoice = '') {
  const fd = new FormData()
  fd.append('name', name)
  fd.append('level', String(level ?? 1))
  fd.append('source', source || '')
  fd.append('mp_cost', mpCost === null || mpCost === undefined ? '' : String(mpCost))
  fd.append('hp_cost', hpCost === null || hpCost === undefined ? '' : String(hpCost))
  fd.append('origin', origin || '')
  fd.append('description', description || '')
  if (iconFile) fd.append('icon', iconFile)
  else if (iconChoice) fd.append('icon_choice', iconChoice)
  return fd
}

// Gear: created on Character Assets, equipped from a character's Gear tab.
export const gearApi = {
  async list() {
    const res = await fetch(`${BASE}/gear`)
    if (!res.ok) throw apiError('failed to load gear')
    return res.json()
  },
  async create(formData) {
    const res = await fetch(`${BASE}/gear`, { method: 'POST', body: formData })
    if (!res.ok) throw apiError((await res.text()).trim() || 'failed to create gear')
    return res.json()
  },
  async update(id, formData) {
    const res = await fetch(`${BASE}/gear/${id}`, { method: 'PUT', body: formData })
    if (!res.ok) throw apiError((await res.text()).trim() || 'failed to update gear')
    return res.json()
  },
  async delete(id) {
    const res = await fetch(`${BASE}/gear/${id}`, { method: 'DELETE' })
    if (!res.ok) throw apiError('failed to delete gear')
    return res.json()
  },
}

export function buildGearFormData({ name, slot, weight, modifiers }, iconFile, iconChoice = '') {
  const fd = new FormData()
  fd.append('name', name)
  fd.append('slot', slot)
  fd.append('weight', weight === null || weight === undefined ? '' : String(weight))
  fd.append('modifiers', JSON.stringify(modifiers || {}))
  if (iconFile) fd.append('icon', iconFile)
  else if (iconChoice) fd.append('icon_choice', iconChoice)
  return fd
}

// Mirrors backend/stats.go's specialBaseKeys exactly — every stat that
// has a typed base value, sent as its own form field.
export const SPECIAL_BASE_KEYS = [
  'sanity', 'heat_threshold', 'cold_threshold',
  'resist_water', 'resist_fire', 'resist_wind', 'resist_earth', 'resist_ice', 'resist_thunder', 'resist_impact',
  'cooking', 'crafting', 'alchemy', 'hunting', 'gathering', 'farming', 'blessing', 'enchanting', 'smithing',
]

// Builds the multipart form the backend's updateVersionHandler expects out
// of the flat reactive form object the edit page works with, plus the
// special-stat bases (a separate flat object keyed by SPECIAL_BASE_KEYS).
export function buildVersionFormData(form, specialBases, pictureFile, spellIds, equippedGear) {
  const fd = new FormData()
  const fields = [
    'version_date', 'version_reference', 'name', 'nickname', 'level',
    'class', 'subclass', 'specialization',
    'gender', 'race', 'height', 'weight', 'body_type', 'age', 'human_birth_date',
    'blood_type', 'born_in', 'nation', 'born_in_location_id', 'nation_location_id', 'birth_date', 'deaths', 'chapter_id',
    'description', 'bio', 'speech_mannerisms', 'status',
    'vit', 'def', 'res', 'str', 'dex', 'intel', 'wis', 'agl',
  ]
  for (const field of fields) {
    fd.append(field, form[field] ?? '')
  }
  for (const key of SPECIAL_BASE_KEYS) {
    fd.append(key, specialBases[key] ?? 0)
  }
  // Omitted entirely when not provided, which tells the backend to leave
  // the character's spells untouched.
  if (Array.isArray(spellIds)) fd.append('spell_ids', JSON.stringify(spellIds))
  // Same for gear: { equipment slot: gear id }.
  if (equippedGear) {
    fd.append('gear', JSON.stringify(Object.fromEntries(Object.entries(equippedGear).map(([slot, g]) => [slot, g.id]))))
  }
  // The character's tags (shared by all its versions).
  if (Array.isArray(form.tags)) fd.append('tags', JSON.stringify(form.tags))
  if (pictureFile) fd.append('picture', pictureFile)
  return fd
}

// The Map page: every location and hex in one payload, location CRUD, and
// batched paint/erase. Errors carry the server's message so forms can show
// things like "that color is already used by another kingdom".
async function mapRequest(path, options, fallback) {
  const res = await fetch(`${BASE}${path}`, options)
  if (!res.ok) throw apiError((await res.text()).trim() || fallback)
  return res.json()
}

const jsonBody = (method, body) => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})

export const mapApi = {
  get: () => mapRequest('/map', undefined, 'failed to load the map'),
  createLocation: (payload) =>
    mapRequest('/map/locations', jsonBody('POST', payload), 'failed to create location'),
  updateLocation: (id, payload) =>
    mapRequest(`/map/locations/${id}`, jsonBody('PUT', payload), 'failed to update location'),
  deleteLocation: (id) =>
    mapRequest(`/map/locations/${id}`, { method: 'DELETE' }, 'failed to delete location'),
  // hexes is an array of [q, r] pairs
  paint: (locationId, hexes) =>
    mapRequest('/map/paint', jsonBody('POST', { location_id: locationId, hexes }), 'failed to paint'),
  // locationId may be null, which erases everything covering those hexes
  erase: (locationId, hexes) =>
    mapRequest('/map/erase', jsonBody('POST', { location_id: locationId, hexes }), 'failed to erase'),
}

// ---- Events ------------------------------------------------------------------

async function eventRequest(path, options, fallback) {
  const res = await fetch(`${BASE}${path}`, options)
  if (!res.ok) throw apiError((await res.text()).trim() || fallback)
  return res.json()
}

// Builds the multipart body shared by create and update. tags and
// characterIds are sent as JSON arrays; picture is a File or null.
export function buildEventFormData({ name, description, eventDate, locationId, chapterId, tags, characterIds }, pictureFile, removePicture) {
  const fd = new FormData()
  fd.append('chapter_id', chapterId ?? '')
  fd.append('name', name ?? '')
  fd.append('description', description ?? '')
  fd.append('event_date', eventDate ?? '')
  fd.append('location_id', locationId ?? '')
  fd.append('tags', JSON.stringify(tags ?? []))
  fd.append('character_ids', JSON.stringify(characterIds ?? []))
  if (pictureFile) fd.append('picture', pictureFile)
  if (removePicture) fd.append('remove_picture', '1')
  return fd
}

export const eventsApi = {
  // filters: { tag, q, characterId, locationId }
  list: (filters = {}) => {
    const p = new URLSearchParams()
    if (filters.tag) p.set('tag', filters.tag)
    if (filters.chapterId) p.set('chapter_id', filters.chapterId)
    if (filters.q) p.set('q', filters.q)
    if (filters.characterId) p.set('character_id', filters.characterId)
    if (filters.locationId) p.set('location_id', filters.locationId)
    const qs = p.toString()
    return eventRequest(`/events${qs ? `?${qs}` : ''}`, undefined, 'failed to load events')
  },
  get: (id) => eventRequest(`/events/${id}`, undefined, 'failed to load event'),
  create: (formData) => eventRequest('/events', { method: 'POST', body: formData }, 'failed to create event'),
  update: (id, formData) =>
    eventRequest(`/events/${id}`, { method: 'PUT', body: formData }, 'failed to save event'),
  remove: (id) => eventRequest(`/events/${id}`, { method: 'DELETE' }, 'failed to delete event'),
  // Finds (or creates) the details record behind an imported timeline
  // node. sourceType is 'character' (birth) or 'location' (founding).
  fromSource: (sourceType, sourceId) =>
    eventRequest(
      '/events/from-source',
      jsonBody('POST', { source_type: sourceType, source_id: sourceId }),
      'failed to open event details',
    ),
  tags: () => eventRequest('/event-tags', undefined, 'failed to load tags'),
}

// Every dated event, birth and founding on one number line, already sorted.
export const timelineApi = {
  get: () => eventRequest('/timeline', undefined, 'failed to load the timeline'),
}

export async function fetchCharacterOptions() {
  const res = await fetch(`${BASE}/character-options`)
  if (!res.ok) return []
  return res.json()
}

// ---- Relations, factions, chapters ---------------------------------------------

// Messages these endpoints can send back (backend relations.go, factions.go,
// chapters.go); listed so they get translated.
void [
  tr("a character can't be related to themselves"), tr('pick two characters'),
  tr('a custom relation needs its wording'), tr('that wording is too long'), tr('unknown kind of relation'),
  tr('dates must look like DD-MM-YYYY (or just a year)'), tr('the end date is before the start date'),
  tr('failed to save the relation'), tr('failed to delete the relation'), tr('relation not found'),
  tr('name is required'), tr('a faction with that name already exists'), tr('invalid color'),
  tr("a faction can't sit inside itself"), tr('invalid member list'), tr('too many members'),
  tr('failed to save the faction'), tr('failed to delete the faction'), tr('faction not found'),
  tr('failed to load factions'), tr('a title is required'), tr('that text is too long'),
  tr('failed to save the chapter'), tr('failed to delete the chapter'), tr('chapter not found'),
  tr('failed to load chapters'), tr('failed to save the volume'), tr('failed to delete the volume'),
  tr('volume not found'), tr('failed to save the order'), tr('invalid request'), tr('failed to load relations'),
]

const request = eventRequest

export const fetchConnections = (characterId) =>
  request(`/characters/${characterId}/connections`, undefined, 'failed to load relations')

export const relationsApi = {
  create: (payload) => request('/relations', jsonBody('POST', payload), 'failed to save the relation'),
  update: (id, payload) => request(`/relations/${id}`, jsonBody('PUT', payload), 'failed to save the relation'),
  remove: (id) => request(`/relations/${id}`, { method: 'DELETE' }, 'failed to delete the relation'),
}

export const factionsApi = {
  list: () => request('/factions', undefined, 'failed to load factions'),
  get: (id) => request(`/factions/${id}`, undefined, 'faction not found'),
  // formData: name, description, color, hq_location_id, parent_id,
  // founding_date, members (JSON), and optionally picture / remove_picture.
  save: (id, formData) =>
    request(id ? `/factions/${id}` : '/factions', { method: id ? 'PUT' : 'POST', body: formData }, 'failed to save the faction'),
  remove: (id) => request(`/factions/${id}`, { method: 'DELETE' }, 'failed to delete the faction'),
}

export const chaptersApi = {
  list: () => request('/chapters', undefined, 'failed to load chapters'),
  get: (id) => request(`/chapters/${id}`, undefined, 'chapter not found'),
  create: (payload) => request('/chapters', jsonBody('POST', payload), 'failed to save the chapter'),
  update: (id, payload) => request(`/chapters/${id}`, jsonBody('PUT', payload), 'failed to save the chapter'),
  remove: (id) => request(`/chapters/${id}`, { method: 'DELETE' }, 'failed to delete the chapter'),
  // { volumes: [ids], chapters: [{ id, volume_id }] }, both in reading order
  reorder: (order) => request('/chapters/order', jsonBody('PUT', order), 'failed to save the order'),
  createVolume: (title) => request('/volumes', jsonBody('POST', { title }), 'failed to save the volume'),
  renameVolume: (id, title) => request(`/volumes/${id}`, jsonBody('PUT', { title }), 'failed to save the volume'),
  removeVolume: (id) => request(`/volumes/${id}`, { method: 'DELETE' }, 'failed to delete the volume'),
}
