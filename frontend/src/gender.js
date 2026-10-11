// A character's gender: unknown (empty), Male, Female, or whatever the writer
// types. Male and Female are stored in English and shown in the interface's
// language; typed text is the story's own and shown as written.

import { t, tr } from './i18n'

export const GENDERS = [
  { key: '', label: tr('Unknown') },
  { key: 'Male', label: tr('Male') },
  { key: 'Female', label: tr('Female') },
]

// Which of the three a stored value is, or null for typed text.
export function genderKind(value) {
  const v = String(value || '').trim().toLowerCase()
  if (!v) return ''
  if (v === 'male') return 'Male'
  if (v === 'female') return 'Female'
  return null
}

export function genderLabel(value) {
  const kind = genderKind(value)
  return kind ? t(kind) : kind === '' ? '' : value
}
