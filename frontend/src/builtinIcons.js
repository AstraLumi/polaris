// Built-in icons: simple line drawings on a 24x24 grid, drawn in the
// theme's accent colour. The database stores only "builtin:<id>" (see
// backend/icons.go, which keeps a matching allowlist). Used for spells now
// and meant for any other "pick an icon" spot.

export const BUILTIN_PREFIX = 'builtin:'

import { tr } from './i18n'

// `label` is an English UI string, translated where it's shown.
export const BUILTIN_ICONS = [
  { id: 'flame', label: tr('Flame'), d: ['M12 3c1 3 5 5 5 10a5 5 0 0 1-10 0c0-2 1-3 2-4 0 2 1 3 2 3 0-3 0-6 1-9z'] },
  { id: 'water', label: tr('Water'), d: ['M12 3s6 6.5 6 11a6 6 0 0 1-12 0c0-4.5 6-11 6-11z'] },
  { id: 'leaf', label: tr('Nature'), d: ['M5 19c0-9 5-14 14-14 0 9-5 14-14 14z', 'M5 19l8-8'] },
  { id: 'bolt', label: tr('Lightning'), d: ['M13 2L5 14h6l-1 8 8-12h-6z'] },
  { id: 'ice', label: tr('Ice'), d: ['M12 2v20', 'M3.3 7l17.4 10', 'M3.3 17L20.7 7', 'M9.5 4l2.5 2 2.5-2', 'M9.5 20l2.5-2 2.5 2'] },
  { id: 'wind', label: tr('Wind'), d: ['M3 8h10a3 3 0 1 0-3-3', 'M3 12h15a3 3 0 1 1-3 3', 'M3 16h8'] },
  { id: 'earth', label: tr('Earth'), d: ['M3 20l6-11 4 6 3-4 5 9z'] },
  { id: 'sun', label: tr('Light'), d: ['M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z', 'M12 2v2.5', 'M12 19.5V22', 'M2 12h2.5', 'M19.5 12H22', 'M4.9 4.9l1.8 1.8', 'M17.3 17.3l1.8 1.8', 'M4.9 19.1l1.8-1.8', 'M17.3 6.7l1.8-1.8'] },
  { id: 'moon', label: tr('Shadow'), d: ['M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5z'] },
  { id: 'sparkles', label: tr('Arcane'), d: ['M11 3l1.8 5.2L18 10l-5.2 1.8L11 17l-1.8-5.2L4 10l5.2-1.8z', 'M19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z'] },
  { id: 'heart', label: tr('Healing'), d: ['M12 20s-8-5-8-11a4.5 4.5 0 0 1 8-2.5A4.5 4.5 0 0 1 20 9c0 6-8 11-8 11z'] },
  { id: 'shield', label: tr('Ward'), d: ['M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z'] },
  { id: 'skull', label: tr('Death'), d: ['M12 3a8 8 0 0 0-8 8c0 3 1.5 4.5 3 5.5V20h10v-3.5c1.5-1 3-2.5 3-5.5a8 8 0 0 0-8-8z', 'M9 12h.01', 'M15 12h.01', 'M10 20v-2.5', 'M14 20v-2.5'] },
  { id: 'eye', label: tr('Vision'), d: ['M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12z', 'M12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5z'] },
  { id: 'sword', label: tr('Blade'), d: ['M12 2l2.5 3.5V15h-5V5.5z', 'M6.5 15h11', 'M12 15v5', 'M10 21h4'] },
  { id: 'tome', label: tr('Knowledge'), d: ['M12 6c-2-1.5-5-2-8-2v14c3 0 6 .5 8 2 2-1.5 5-2 8-2V4c-3 0-6 .5-8 2z', 'M12 6v14'] },
  { id: 'star', label: tr('Star'), d: ['M12 2l2.4 6.6L21 9.3l-5.2 4.4L17.5 21 12 17.2 6.5 21l1.7-7.3L3 9.3l6.6-.7z'] },
]

// Extra icons that are only offered for stories (backend/stories.go keeps the
// matching allowlist). They are drawn like the rest.
const STORY_ONLY_ICONS = [
  { id: 'compass', label: tr('Compass'), d: ['M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z', 'M15.5 8.5l-2 5-5 2 2-5z'] },
  { id: 'castle', label: tr('Castle'), d: ['M4 21V5h2v2h2V5h2v2h2V5h2v2h2V5h2v2h2v14z', 'M10 21v-5a2 2 0 0 1 4 0v5'] },
  { id: 'crown', label: tr('Crown'), d: ['M3 8l4.5 4L12 5l4.5 7L21 8l-2 11H5z'] },
  { id: 'mountain', label: tr('Mountain'), d: ['M2 20L10 6l4 7 2-3 6 10z', 'M7.5 10.5l2.5 2 2-2'] },
]

// Icons drawn only for gear (backend/icons.go keeps the matching allowlist).
// 'sword' and 'shield' are the spell icons, reused.
const GEAR_ONLY_ICONS = [
  { id: 'helmet', label: tr('Helmet'), d: ['M4 19v-5a8 8 0 0 1 16 0v5z', 'M4 14h16', 'M12 6v8'] },
  { id: 'mask', label: tr('Mask'), d: ['M4 8c3-2 13-2 16 0v4c0 4-3 7-8 7s-8-3-8-7z', 'M7.5 11.5H10.5', 'M13.5 11.5H16.5'] },
  { id: 'necklace', label: tr('Necklace'), d: ['M5 4c0 8 3 12 7 12s7-4 7-12', 'M12 16l-2.5 3 2.5 3 2.5-3z'] },
  { id: 'cape', label: tr('Cape'), d: ['M8 4h8l4 16c-2.5 1-5 1-8-.5-3 1.5-5.5 1.5-8 .5z', 'M8 4c0 2 2 3 4 3s4-1 4-3'] },
  { id: 'armor', label: tr('Armor'), d: ['M8 4L3 7l2 4 3-1v10h8V10l3 1 2-4-5-3c-1 2-2.5 3-4 3S9 6 8 4z'] },
  { id: 'glove', label: tr('Glove'), d: ['M6 12V7', 'M9.5 11V4.5', 'M13 11V5', 'M16.5 12V8', 'M5 12h13.5v3.5c0 3-2 5-5 5H10c-3 0-5-2-5-5z', 'M8 20.5V22', 'M15 20.5V22'] },
  { id: 'pants', label: tr('Pants'), d: ['M7 3h10l1 18h-4l-2-10-2 10H6z'] },
  { id: 'boots', label: tr('Boots'), d: ['M8 3h6v8l6 3v6H5v-5l3-2z', 'M8 7h6'] },
  { id: 'ring', label: tr('Ring'), d: ['M12 9a6 6 0 1 0 0 12 6 6 0 0 0 0-12z', 'M12 9L9.5 5h5z'] },
  { id: 'bow', label: tr('Bow'), d: ['M6 3c9 2 12 7 12 9s-3 7-12 9', 'M6 3v18', 'M6 12h14', 'M17 9l3 3-3 3'] },
  { id: 'staff', label: tr('Staff'), d: ['M12 2.5l3 3.5-3 3.5-3-3.5z', 'M12 9.5V22', 'M9.5 22h5'] },
  { id: 'axe', label: tr('Axe'), d: ['M10 3v18', 'M10 5C6 4 3 6 3 9c1 3 4 4 7 3', 'M10 5c4-1 8 .5 8 4 0 3-4 4.5-8 3'] },
  { id: 'dagger', label: tr('Dagger'), d: ['M12 2l2.5 8v4h-5v-4z', 'M7.5 14h9', 'M12 14v5', 'M10.5 21h3'] },
  { id: 'hammer', label: tr('Hammer'), d: ['M5 5h11v6H5z', 'M10.5 11v10', 'M16 8h3'] },
]

const GEAR_ICON_IDS = ['helmet', 'mask', 'necklace', 'cape', 'armor', 'glove', 'pants', 'boots', 'ring', 'sword', 'shield', 'bow', 'staff', 'axe', 'dagger', 'hammer']
const GEAR_LABELS = { sword: tr('Sword'), shield: tr('Shield') }
export const GEAR_ICONS = GEAR_ICON_IDS.map((id) => {
  const icon = GEAR_ONLY_ICONS.find((i) => i.id === id) || BUILTIN_ICONS.find((i) => i.id === id)
  return GEAR_LABELS[id] ? { ...icon, label: GEAR_LABELS[id] } : icon
})

// What the story picker offers, in display order.
const STORY_ICON_IDS = ['tome', 'star', 'compass', 'castle', 'crown', 'sword', 'shield', 'mountain', 'flame', 'moon', 'skull', 'sparkles']
export const STORY_ICONS = STORY_ICON_IDS.map(
  (id) => STORY_ONLY_ICONS.find((i) => i.id === id) || BUILTIN_ICONS.find((i) => i.id === id),
)

export function isBuiltinIcon(value) {
  return typeof value === 'string' && value.startsWith(BUILTIN_PREFIX)
}

export function builtinIconFor(value) {
  if (!isBuiltinIcon(value)) return null
  const id = value.slice(BUILTIN_PREFIX.length)
  return BUILTIN_ICONS.find((i) => i.id === id) || STORY_ONLY_ICONS.find((i) => i.id === id) || GEAR_ONLY_ICONS.find((i) => i.id === id) || null
}
