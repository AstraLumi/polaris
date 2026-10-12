import { tr, t } from './i18n'
import { STORY_API } from './stories'
import { mapPath } from './navigation'

// The wiki has no pages of its own: the server builds an article for every
// character, location, event and asset (see backend/wiki.go) and stores only
// the extra text added here. The labels below are the English text the server
// refers to by key; tr() marks them for translation.

export const TYPE_ORDER = [
  'character', 'location', 'faction', 'event', 'class', 'subclass', 'specialization', 'race', 'body_type', 'spell', 'gear', 'creature', 'lore',
]

export const TYPE_LABELS = {
  character: tr('Character'),
  location: tr('Location'),
  faction: tr('Faction'),
  event: tr('Event'),
  class: tr('Class'),
  subclass: tr('Subclass'),
  specialization: tr('Specialization'),
  race: tr('Race'),
  body_type: tr('Body type'),
  spell: tr('Spell'),
  gear: tr('Gear'),
  creature: tr('Creature'),
  lore: tr('Lore'),
}

export const TYPE_PLURALS = {
  character: tr('Characters'),
  location: tr('Locations'),
  faction: tr('Factions'),
  event: tr('Events'),
  class: tr('Classes'),
  subclass: tr('Subclasses'),
  specialization: tr('Specializations'),
  race: tr('Races'),
  body_type: tr('Body types'),
  spell: tr('Spells'),
  gear: tr('Gear'),
  creature: tr('Creatures'),
  lore: tr('Lore'),
}

// The fixed markdown boxes (which ones an article has comes from the server).
export const FIELD_LABELS = {
  appearance: tr('Appearance'),
  personality: tr('Personality'),
  background: tr('Background'),
  relationships: tr('Relationships'),
  abilities: tr('Abilities'),
  trivia: tr('Trivia'),
  geography: tr('Geography'),
  history: tr('History'),
  culture: tr('Culture'),
  inhabitants: tr('Inhabitants'),
  points_of_interest: tr('Points of interest'),
  course: tr('Course of events'),
  aftermath: tr('Aftermath'),
  lore: tr('Lore'),
  training: tr('Training'),
  notable_members: tr('Notable members'),
  notes: tr('Notes'),
  usage: tr('Usage'),
  origin: tr('Origin'),
  behavior: tr('Behaviour'),
  habitat: tr('Habitat'),
}

// Infobox rows that come from the story itself.
export const FACT_LABELS = {
  nickname: tr('Nickname'),
  also_known_as: tr('Also known as'),
  kingdom: tr('Kingdom'),
  status: tr('Status'),
  race: tr('Race'),
  gender: tr('Gender'),
  body_type: tr('Body type'),
  class: tr('Class'),
  subclass: tr('Subclass'),
  specialization: tr('Specialization'),
  born: tr('Born'),
  human_birth_date: tr('Human birth date'),
  age: tr('Age'),
  blood_type: tr('Blood type'),
  born_in: tr('Born in'),
  nation: tr('Nation'),
  type: tr('Type'),
  part_of: tr('Part of'),
  hemisphere: tr('Hemisphere'),
  latitude: tr('Latitude'),
  climate_zone: tr('Climate zone'),
  founded: tr('Founded'),
  date: tr('Date'),
  location: tr('Location'),
  tags: tr('Tags'),
  source: tr('Source'),
  origin: tr('Origin'),
  native_to: tr('Native to'),
  level: tr('Level'),
  slot: tr('Slot'),
  settlement: tr('Settlement'),
  capital: tr('Capital'),
  headquarters: tr('Headquarters'),
}

// How the infobox groups its facts, top to bottom. Facts that aren't listed
// here stay at the top, with Kind, in the order the server sends them; a
// group with none of its facts filled in isn't shown.
export const FACT_GROUPS = [
  { label: '', keys: ['nickname', 'also_known_as', 'status', 'tags'] },
  { label: tr('Personal'), keys: ['gender', 'age', 'race', 'body_type', 'blood_type'] },
  { label: tr('Birth'), keys: ['born', 'born_in', 'nation', 'human_birth_date'] },
  { label: tr('Abilities'), keys: ['class', 'subclass', 'specialization'] },
  { label: tr('Geography'), keys: ['hemisphere', 'latitude', 'climate_zone'] },
]

// Words the server sends as an infobox value.
export const WORD_LABELS = [
  tr('Kingdom'), tr('Major location'), tr('Minor location'), tr('Capital'), tr('City'),
  tr('Alive'), tr('Missing'), tr('Dead'),
  tr('Monster'), tr('Animal'), tr('Construct'),
  tr('Northern hemisphere'), tr('Southern hemisphere'), tr('Both hemispheres'), tr('On the equator'),
  tr('Tropical'), tr('Temperate'), tr('Polar'),
  tr('Region'), tr('Ocean'),
]

// A 'words' value: several of the words above, joined by '|'.
export const factWords = (value) => value.split('|').map((w) => t(w)).join(', ')

// Related lists.
export const GROUP_LABELS = {
  events: tr('Events'),
  people: tr('Involved people'),
  places: tr('Places within'),
  events_here: tr('Events here'),
  born_here: tr('Born here'),
  nation_members: tr('People of this nation'),
  spells_from: tr('Spells from here'),
  native_races: tr('Races native here'),
  native_specializations: tr('Specializations native here'),
  creatures_here: tr('Creatures native here'),
  related_creatures: tr('Related creatures'),
  subclasses: tr('Subclasses'),
  specializations: tr('Specializations'),
  spells: tr('Spells'),
  members: tr('Members'),
  former_members: tr('Former members'),
  subfactions: tr('Factions within'),
  factions: tr('Factions'),
  factions_here: tr('Factions based here'),
  borders: tr('Borders'),
  relations: tr('Relations'),
  classes: tr('Classes'),
  links_to: tr('Links to'),
}

// Text that already exists on the sheet or event and is shown read-only.
export const SHEET_LABELS = {
  description: tr('Description'),
  bio: tr('Biography'),
  speech: tr('Speech and mannerisms'),
}

export const wikiPath = (type, id) => `/wiki/${type}/${id}`

// Where the facts of an article are edited.
export function sourcePath(type, id) {
  if (type === 'character') return `/characters/${id}`
  if (type === 'event') return `/events/${id}`
  if (type === 'location') return mapPath(id)
  if (type === 'faction') return '/factions'
  if (type === 'creature') return '/bestiary'
  return '/assets'
}

export function sourceLabel(type) {
  if (type === 'character') return t('character sheet')
  if (type === 'event') return t('event page')
  if (type === 'location') return t('map')
  if (type === 'faction') return t('Factions page')
  if (type === 'creature') return t('Bestiary')
  return t('Character Assets page')
}

// Messages the server can send back (backend/wiki.go); listed so they get translated.
export const SERVER_MESSAGES = [
  tr('article not found'),
  tr('that text is too long'),
  tr('every section needs a title'),
  tr('every info row needs a label'),
  tr('too many sections'),
  tr('too many info rows'),
  tr('invalid request'),
  tr('failed to save the article'),
  tr('failed to load the article'),
  tr('the article needs a title'),
  tr("that title can't be used"),
  tr('failed to delete the article'),
  tr('every dated row needs a readable date'),
  tr('failed to save the picture'),
  tr('failed to delete the picture'),
  tr('pick at least one picture'),
  tr('picture not found'),
  tr('there is no year 0 — the year before 1 is -1'),
]

async function request(path, options, fallback) {
  const res = await fetch(`${STORY_API}${path}`, options)
  if (!res.ok) throw new Error(t((await res.text()).trim() || fallback))
  return res.json()
}

export const wikiApi = {
  list: () => request('/wiki', undefined, tr('failed to load the wiki')),
  // Every article and every connection between two (backend/wikigraph.go).
  graph: () => request('/wiki/graph', undefined, tr('failed to load the graph')),
  get: (type, id) => request(`/wiki/${type}/${id}`, undefined, tr('failed to load the article')),
  save: (type, id, payload) =>
    request(
      `/wiki/${type}/${id}`,
      { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) },
      tr('failed to save the article'),
    ),
  // Lore articles (free-form pages): create with id null, or rename and set
  // the picture. FormData: name, picture, remove_picture.
  saveLore: (id, formData) =>
    request(id ? `/lore/${id}` : '/lore', { method: id ? 'PUT' : 'POST', body: formData }, tr('failed to save the article')),
  deleteLore: (id) => request(`/lore/${id}`, { method: 'DELETE' }, tr('failed to delete the article')),
  // An article's gallery (backend/wikigallery.go); each call answers with
  // the whole gallery as it now is.
  galleryAdd: (type, id, files) => {
    const fd = new FormData()
    for (const f of files) fd.append('pictures', f)
    return request(`/wiki/${type}/${id}/gallery`, { method: 'POST', body: fd }, tr('failed to save the picture'))
  },
  galleryOrder: (type, id, ids) =>
    request(
      `/wiki/${type}/${id}/gallery`,
      { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ids }) },
      tr('failed to save the order'),
    ),
  galleryCaption: (gid, caption) =>
    request(
      `/wiki-gallery/${gid}`,
      { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ caption }) },
      tr('failed to save the picture'),
    ),
  galleryRemove: (gid) => request(`/wiki-gallery/${gid}`, { method: 'DELETE' }, tr('failed to delete the picture')),
}

// Same rules as resolveWikiLink in backend/wiki.go: [[Name]], or
// [[type:Name]] when two kinds of article share a name.
const norm = (s) => s.replace(/_/g, ' ').toLowerCase().split(/\s+/).filter(Boolean).join(' ')

export function makeLinkResolver(items) {
  const index = new Map()
  const add = (name, it) => {
    const k = norm(name)
    if (!index.has(k)) index.set(k, [])
    index.get(k).push(it)
  }
  for (const it of items) add(it.name, it)
  // A character's former names, after every real name (as on the server).
  for (const it of items) for (const a of it.aliases || []) add(a, it)
  return (inner) => {
    inner = inner.trim()
    const colon = inner.indexOf(':')
    if (colon > 0) {
      const prefix = norm(inner.slice(0, colon))
      const type = TYPE_ORDER.find((k) => norm(k) === prefix)
      if (type) return (index.get(norm(inner.slice(colon + 1))) || []).find((x) => x.type === type) || null
    }
    return (index.get(norm(inner)) || [])[0] || null
  }
}
