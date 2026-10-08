import { t, tr } from './i18n'

// Only shows the costs that are actually set: "12 MP", "5 HP",
// "12 MP · 5 HP", or "Free" when neither is.
export function formatSpellCost(spell) {
  const parts = []
  if (spell.mp_cost) parts.push(`${spell.mp_cost} MP`)
  if (spell.hp_cost) parts.push(`${spell.hp_cost} HP`)
  return parts.length ? parts.join(' · ') : t('Free')
}

export function formatSpellLevel(level) {
  return t('Lv. {level}', { level: String(level ?? 1).padStart(2, '0') })
}

const SOURCE_TYPE_LABELS = {
  class: tr('Class'),
  subclass: tr('Subclass'),
  specialization: tr('Specialization'),
}

// "Class · Knight", or '' when the spell has no source.
export function formatSpellSource(spell) {
  if (!spell.source_type || !spell.source_name) return ''
  const type = SOURCE_TYPE_LABELS[spell.source_type]
  return t('{type} · {name}', { type: type ? t(type) : spell.source_type, name: spell.source_name })
}
