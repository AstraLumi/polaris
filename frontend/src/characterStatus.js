// Whether a character is alive, missing or dead at a point in the story
// (stored per version). Separate from the death counter: a character can
// die, come back and still be alive.
import { t, tr } from './i18n'

export const STATUSES = [
  { key: 'alive', label: tr('Alive') },
  { key: 'missing', label: tr('Missing') },
  { key: 'dead', label: tr('Dead') },
]

export const statusLabel = (key) => t(STATUSES.find((s) => s.key === key)?.label ?? 'Alive')
