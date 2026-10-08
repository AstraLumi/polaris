// Gear: slot types, equipment slots and small helpers shared by the Character
// Assets page and the character sheet. Mirrors backend/gear.go.

import { tr, t } from './i18n'
import { BUILTIN_PREFIX } from './builtinIcons'
import { MODIFIER_GROUPS } from './api'

// What a piece of gear IS. `icon` is the built-in icon shown when the gear has
// none of its own.
export const GEAR_SLOT_TYPES = [
  { id: 'helmet', label: tr('Helmet'), icon: 'helmet' },
  { id: 'face', label: tr('Face'), icon: 'mask' },
  { id: 'necklace', label: tr('Necklace'), icon: 'necklace' },
  { id: 'cape', label: tr('Cape'), icon: 'cape' },
  { id: 'torso', label: tr('Torso'), icon: 'armor' },
  { id: 'glove', label: tr('Glove'), icon: 'glove' },
  { id: 'ring', label: tr('Ring'), icon: 'ring' },
  { id: 'pants', label: tr('Pants'), icon: 'pants' },
  { id: 'boots', label: tr('Boots'), icon: 'boots' },
  { id: 'mainhand', label: tr('Mainhand'), icon: 'sword' },
  { id: 'offhand', label: tr('Offhand'), icon: 'shield' },
]

// Where gear is worn on a character. Two glove slots both take a glove.
export const EQUIP_SLOTS = [
  { key: 'helmet', type: 'helmet', label: tr('Helmet') },
  { key: 'face', type: 'face', label: tr('Face') },
  { key: 'necklace', type: 'necklace', label: tr('Necklace') },
  { key: 'cape', type: 'cape', label: tr('Cape') },
  { key: 'torso', type: 'torso', label: tr('Torso') },
  { key: 'pants', type: 'pants', label: tr('Pants') },
  { key: 'glove1', type: 'glove', label: tr('Glove 1') },
  { key: 'glove2', type: 'glove', label: tr('Glove 2') },
  { key: 'ring1', type: 'ring', label: tr('Ring 1') },
  { key: 'ring2', type: 'ring', label: tr('Ring 2') },
  { key: 'boots', type: 'boots', label: tr('Boots') },
  { key: 'mainhand', type: 'mainhand', label: tr('Mainhand') },
  { key: 'offhand', type: 'offhand', label: tr('Offhand') },
]

export const slotTypeDef = (id) => GEAR_SLOT_TYPES.find((s) => s.id === id) || null
export const slotTypeLabel = (id) => (slotTypeDef(id) ? t(slotTypeDef(id).label) : id)

// The icon to draw for a piece of gear: its own, else its slot type's.
export function gearIconSrc(gear) {
  if (gear?.icon_path) return gear.icon_path
  const def = slotTypeDef(gear?.slot)
  return def ? BUILTIN_PREFIX + def.icon : ''
}

// ---- Stat bonuses -----------------------------------------------------------

const STAT_ORDER = []
const STAT_LABELS = {}
for (const group of MODIFIER_GROUPS) {
  for (const [key, label] of group.keys) {
    STAT_ORDER.push(key)
    STAT_LABELS[key] = label
  }
}

export const statLabel = (key) => (STAT_LABELS[key] ? t(STAT_LABELS[key]) : key)

const round = (n) => Math.round(n * 100) / 100

// [{ key, text }] for a { stat: value } map, in the order stats are listed
// everywhere else, e.g. "+5 Attack".
export function formatBonuses(modifiers) {
  return STAT_ORDER.filter((k) => modifiers?.[k])
    .map((key) => {
      const v = round(modifiers[key])
      return { key, value: v, text: `${v > 0 ? '+' : '\u2212'}${Math.abs(v)} ${statLabel(key)}` }
    })
}

// Adds up the bonuses of everything equipped (a piece worn in two slots counts
// twice, matching how the server computes stats).
export function totalBonuses(equipped) {
  const total = {}
  for (const g of Object.values(equipped || {})) {
    for (const [k, v] of Object.entries(g.modifiers || {})) total[k] = (total[k] || 0) + v
  }
  return total
}

export const totalWeight = (equipped) =>
  Object.values(equipped || {}).reduce((sum, g) => sum + (Number(g.weight) || 0), 0)

export const formatKg = (n) => `${(Math.round((Number(n) || 0) * 100) / 100).toFixed(2)} kg`
