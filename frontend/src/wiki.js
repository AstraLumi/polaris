import { tr, t } from './i18n'
import { STORY_API } from './stories'
import { mapPath } from './navigation'

// The wiki has no pages of its own: the server builds an article for every
// character, location, event and asset (see backend/wiki.go) and stores only
// the extra text added here. The labels below are the English text the server
// refers to by key; tr() marks them for translation.

export const TYPE_ORDER = [
  'character', 'location', 'faction', 'event', 'class', 'subclass', 'specialization', 'race', 'body_type', 'spell', 'gear',
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
}

// Infobox rows that come from the story itself.
export const FACT_LABELS = {
  nickname: tr('Nickname'),
  status: tr('Status'),
  race: tr('Race'),
  gender: tr('Gender'),
  body_type: tr('Body type'),
  class: tr('Class'),
  subclass: tr('Subclass'),
  specialization: tr('Specialization'),
  born: tr('Born'),
  born_in: tr('Born in'),
  nation: tr('Nation'),
  type: tr('Type'),
  part_of: tr('Part of'),
  founded: tr('Founded'),
  date: tr('Date'),
  location: tr('Location'),
  tags: tr('Tags'),
  source: tr('Source'),
  origin: tr('Origin'),
  slot: tr('Slot'),
  settlement: tr('Settlement'),
  headquarters: tr('Headquarters'),
}

// Words the server sends as an infobox value.
export const WORD_LABELS = [
  tr('Kingdom'), tr('Major location'), tr('Minor location'), tr('Capital'), tr('City'),
  tr('Alive'), tr('Missing'), tr('Dead'),
]

// Related lists.
export const GROUP_LABELS = {
  events: tr('Events'),
  people: tr('Involved people'),
  places: tr('Places within'),
  events_here: tr('Events here'),
  born_here: tr('Born here'),
  nation_members: tr('People of this nation'),
  spells_from: tr('Spells from here'),
  subclasses: tr('Subclasses'),
  specializations: tr('Specializations'),
  spells: tr('Spells'),
  members: tr('Members'),
  former_members: tr('Former members'),
  subfactions: tr('Factions within'),
  factions: tr('Factions'),
  factions_here: tr('Factions based here'),
  relations: tr('Relations'),
  classes: tr('Classes'),
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
  return '/assets'
}

export function sourceLabel(type) {
  if (type === 'character') return t('character sheet')
  if (type === 'event') return t('event page')
  if (type === 'location') return t('map')
  if (type === 'faction') return t('Factions page')
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
]

async function request(path, options, fallback) {
  const res = await fetch(`${STORY_API}${path}`, options)
  if (!res.ok) throw new Error(t((await res.text()).trim() || fallback))
  return res.json()
}

export const wikiApi = {
  list: () => request('/wiki', undefined, tr('failed to load the wiki')),
  get: (type, id) => request(`/wiki/${type}/${id}`, undefined, tr('failed to load the article')),
  save: (type, id, payload) =>
    request(
      `/wiki/${type}/${id}`,
      { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) },
      tr('failed to save the article'),
    ),
}

// Same rules as resolveWikiLink in backend/wiki.go: [[Name]], or
// [[type:Name]] when two kinds of article share a name.
const norm = (s) => s.replace(/_/g, ' ').toLowerCase().split(/\s+/).filter(Boolean).join(' ')

export function makeLinkResolver(items) {
  const index = new Map()
  for (const it of items) {
    const k = norm(it.name)
    if (!index.has(k)) index.set(k, [])
    index.get(k).push(it)
  }
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
