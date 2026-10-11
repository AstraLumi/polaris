// The story's stat system: which stats there are, in which category, and how
// each is calculated (backend/statsystem.go). The character sheet, the bonus
// forms of classes, races and gear, and version comparisons all list what is
// here; Settings → Stats edits it. Loaded once per page (switching stories
// reloads the app) and again after Settings saves.

import { reactive } from 'vue'
import { t, tr } from './i18n'
import { STORY_API } from './stories'

// The categories are fixed; each keeps its behaviour whatever stats it holds.
export const STAT_CATEGORIES = [
  {
    id: 'primary',
    label: tr('Primary Stats'),
    about: tr('Points are spent on these, and their value is the points plus bonuses. Everything else is calculated from them.'),
  },
  {
    id: 'base',
    label: tr('Base Stats'),
    about: tr('Calculated by their formulas. Bonuses from classes, subclasses, specializations, races and body types count once per level; bonuses from gear count once.'),
  },
  { id: 'special', label: tr('Special Stats'), about: tr('Calculated by their formulas. Every bonus counts once.') },
  { id: 'defense', label: tr('Special Defenses'), about: tr('Calculated by their formulas. Every bonus counts once.') },
  { id: 'lifeskill', label: tr('Lifeskill'), about: tr('Calculated by their formulas. Every bonus counts once.') },
]

export const categoryLabel = (id) => t(STAT_CATEGORIES.find((c) => c.id === id)?.label || id)

// The names of the default stats. They are the story's own text, but while
// a story keeps them they read in the interface's language.
export const DEFAULT_STAT_NAMES = [
  tr('Vitality'), tr('Defense'), tr('Resistance'), tr('Strength'), tr('Dexterity'), tr('Intelligence'), tr('Wisdom'), tr('Agility'),
  tr('Attack'), tr('Potency'), tr('Speed'),
  tr('Luck'), tr('Carry Limit'), tr('Fall Damage Threshold'), tr('Sanity'), tr('Heat Threshold'), tr('Cold Threshold'),
  tr('Water'), tr('Fire'), tr('Wind'), tr('Earth'), tr('Ice'), tr('Thunder'), tr('Impact'),
  tr('Cooking'), tr('Crafting'), tr('Alchemy'), tr('Hunting'), tr('Gathering'), tr('Farming'), tr('Blessing'), tr('Enchanting'), tr('Smithing'),
]

export const statSystem = reactive({ points_formula: '[level]', stats: [], loaded: false })

let loading = null

export function applyStatSystem(data) {
  statSystem.points_formula = data.points_formula || ''
  statSystem.stats = data.stats || []
  statSystem.loaded = true
}

// Loads the stat system (once, unless force). Resolves when it's in.
export function loadStatSystem(force = false) {
  if (!STORY_API) return Promise.resolve()
  if (loading && !force) return loading
  loading = fetch(`${STORY_API}/stat-system`)
    .then((r) => (r.ok ? r.json() : null))
    .then((data) => data && applyStatSystem(data))
    .catch(() => {
      loading = null
    })
  return loading
}

export const statsIn = (category, stats = statSystem.stats) => stats.filter((s) => s.category === category)
export const statDef = (key) => statSystem.stats.find((s) => s.key === key) || null

export const statName = (def) => (def ? t(def.name) : '')
// The short name where there is one (VIT), else the name.
export const statShort = (def) => (def ? (def.short_name ? t(def.short_name) : t(def.name)) : '')
export const statLabel = (key) => statShort(statDef(key)) || key

// Whether the sheet asks for a number for this stat: a primary stat's points,
// or a calculated stat whose formula reads its typed value.
export const readsBase = (def) => /\[\s*base\s*\]/.test(`${def?.formula || ''} ${def?.multiplier || ''}`)
export const isTyped = (def) => def?.category === 'primary' || readsBase(def)

// "1.25x", "21.4 kg", "12".
export function formatStat(def, value) {
  const n = Number(value) || 0
  const text = n.toFixed(def?.decimals || 0)
  const unit = def?.unit || ''
  if (!unit) return text
  return unit === 'x' || unit === '%' ? `${text}${unit}` : `${text} ${unit}`
}

// ---- Formulas ---------------------------------------------------------------
// Reading and writing them as blocks is in formulaBlocks.js.

export { parseBlocks, blocksToText, renameRef, keyFromName } from './formulaBlocks'

// Values every formula can read, besides stats.
export const FORMULA_INPUTS = [
  { key: 'level', label: tr('Level'), about: tr("The character's level.") },
  { key: 'age', label: tr('Age'), about: tr("The character's age in years (18 when not set).") },
  { key: 'height', label: tr('Height'), about: tr("The character's height in cm (170 when not set).") },
  { key: 'weight', label: tr('Weight'), about: tr("The character's weight in kg (70 when not set).") },
  { key: 'base', label: tr('Typed value'), about: tr('A number typed on the character sheet for this stat. Using it gives the stat a box on the sheet.') },
]

export const FORMULA_FUNCTIONS = [
  { name: 'min', about: tr('min(a, b, …): the smallest of the values.') },
  { name: 'max', about: tr('max(a, b, …): the largest of the values.') },
  { name: 'round', about: tr('round(x) or round(x, digits): rounds to the nearest whole number, or to that many decimals.') },
  { name: 'floor', about: tr('floor(x): rounds down.') },
  { name: 'ceil', about: tr('ceil(x): rounds up.') },
  { name: 'abs', about: tr('abs(x): the value without its sign.') },
  { name: 'sqrt', about: tr('sqrt(x): the square root.') },
  { name: 'clamp', about: tr('clamp(x, lowest, highest): keeps x between the two.') },
  { name: 'curve', about: tr('curve(x, x1, y1, x2, y2, …): follows straight lines between the points, flat before the first and after the last.') },
]

export const FORMULA_OPERATORS = [
  { op: '+', label: '+', about: tr('Add') },
  { op: '-', label: '−', about: tr('Subtract') },
  { op: '*', label: '×', about: tr('Multiply') },
  { op: '/', label: '÷', about: tr('Divide') },
  { op: '^', label: '^', about: tr('To the power of') },
  { op: '(', label: '(', about: tr('Open a bracket') },
  { op: ')', label: ')', about: tr('Close a bracket') },
  { op: ',', label: ',', about: tr('Separates the values of a function') },
]

// What a formula problem from the server means, in words.
export function issueText(issue, nameOf = (k) => k) {
  const d = issue.detail || ''
  switch (issue.code) {
    case 'incomplete':
      return t('The formula ends too soon.')
    case 'unclosed':
      return t('A bracket is never closed.')
    case 'unopened':
      return t('A bracket is closed that was never opened.')
    case 'bad_number':
      return t('Not a number: {text}', { text: d })
    case 'unknown_function':
      return t('Unknown function: {text}', { text: d })
    case 'arg_count':
      return t('{text} has the wrong number of values.', { text: `${d}( )` })
    case 'unknown_value':
      return d === 'base' ? t('Typed value can\'t be used here.') : t('{text} can\'t be used here.', { text: nameOf(d) })
    case 'loop':
      return t('These stats are calculated from each other in a loop: {text}', {
        text: d.split(' → ').map(nameOf).join(' → '),
      })
    case 'no_name':
      return t('Give this stat a name.')
    case 'min_over_max':
      return t('The lowest value is above the highest.')
    case 'too_many':
      return t('Too many stats (at most {n}).', { n: d })
    case 'bad_key':
    case 'reserved_key':
    case 'duplicate_key':
    case 'bad_category':
      return t('This stat clashes with another. Remove it and add it again.')
    default:
      return t('This doesn\'t fit here: {text}', { text: d })
  }
}
