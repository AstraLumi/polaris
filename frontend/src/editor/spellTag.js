// The hover card's words for a tagged spell ("Tag spell", docs/EDITOR.md):
// what the spell costs, the class it belongs to and its level, from the
// spells API's row (mp_cost / hp_cost null when not set, source_type
// class | subclass | specialization, level). Pure, so it is tested without
// the page. `t` is the interface's translate function.

const SOURCE_LABELS = { class: 'Class', subclass: 'Subclass', specialization: 'Specialization' }

// "12 MP", "5 HP", "12 MP · 5 HP", or "Free" when it costs nothing.
export function spellCost(spell, t) {
  const parts = []
  if (spell?.mp_cost > 0) parts.push(t('{n} MP', { n: spell.mp_cost }))
  if (spell?.hp_cost > 0) parts.push(t('{n} HP', { n: spell.hp_cost }))
  return parts.length ? parts.join(' · ') : t('Free')
}

// { name, cost, source, level } as plain strings; source and level are ''
// when the spell has none.
export function spellCard(spell, t) {
  const kind = SOURCE_LABELS[spell?.source_type]
  return {
    name: spell?.name || '',
    cost: spellCost(spell, t),
    source: kind && spell.source_name ? t('{kind}: {name}', { kind: t(kind), name: spell.source_name }) : '',
    level: Number.isInteger(spell?.level) ? t('Level {level}', { level: spell.level }) : '',
  }
}

// The spells for the picker, by name.
export const spellChoices = (spells) =>
  [...(spells || [])].map((s) => ({ id: s.id, name: s.name, level: s.level })).sort((a, b) => a.name.localeCompare(b.name))
