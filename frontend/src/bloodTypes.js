import { reactive } from 'vue'
import { t, tr } from './i18n'

// The story's blood types (backend/bloodtypes.go), loaded with the settings
// (calendar.js). Each is { name, note }; a built-in type whose note was
// left empty shows the app's own note, below, in the reader's language.
export const bloodTypes = reactive({ list: [] })

export function applyBloodTypes(list) {
  if (Array.isArray(list)) bloodTypes.list = list
}

export const DEFAULT_BLOOD_TYPES = ['A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-'].map((name) => ({ name, note: '' }))

// What the ABO group is said to say about someone in Japan, where blood type
// is read much like a star sign (ketsueki-gata).
const GROUP_NOTES = {
  A: tr('In Japan, type A is said to be earnest, tidy and considerate: a careful planner who keeps the peace, but worries too much and sticks to the rules.'),
  B: tr('In Japan, type B is said to be passionate, curious and free-spirited: deeply absorbed in their own interests, but seen as self-centred and unpredictable.'),
  O: tr('In Japan, type O is said to be a confident, generous leader: optimistic, ambitious and loyal, but careless with details and sometimes overbearing.'),
  AB: tr('In Japan, type AB is said to be cool, rational and hard to read: a creative mix of A and B, sometimes thought eccentric or two-faced.'),
}
const RH_NOTES = {
  '+': tr('Rh-positive, like over 99% of people in Japan.'),
  '-': tr('Rh-negative is rare in Japan (about 1 in 200), so it marks a character as unusual, and makes needing blood a worry.'),
}

// Typed as A+ or A−, with any kind of minus.
function builtinParts(name) {
  const m = /^(AB|A|B|O)\s*([+\-−–])$/i.exec((name || '').trim())
  if (!m) return null
  return { group: m[1].toUpperCase(), rh: m[2] === '+' ? '+' : '-' }
}

export function isBuiltinBloodType(name) {
  return !!builtinParts(name)
}

// The note shown for a blood type: its own, else the built-in one.
export function bloodTypeNote(type) {
  if (!type) return ''
  if (type.note) return type.note
  const parts = builtinParts(type.name)
  if (!parts) return ''
  return `${t(GROUP_NOTES[parts.group])} ${t(RH_NOTES[parts.rh])}`
}

// The note for a blood type by name, looked up in the story's list.
export function bloodTypeNoteFor(name) {
  const key = (name || '').trim().toLowerCase()
  if (!key) return ''
  const found = bloodTypes.list.find((b) => b.name.toLowerCase() === key)
  return bloodTypeNote(found || { name, note: '' })
}
