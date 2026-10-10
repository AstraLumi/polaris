// How a relation between two characters reads from each side. The same
// list is in backend/relations.go (relationWording); keep them in step.
import { t, tr } from './i18n'

export const RELATION_KINDS = [
  { key: 'parent', label: tr('Parent / child'), from: tr('Parent of'), to: tr('Child of') },
  { key: 'sibling', label: tr('Siblings'), from: tr('Sibling of'), to: tr('Sibling of') },
  { key: 'spouse', label: tr('Spouses'), from: tr('Spouse of'), to: tr('Spouse of') },
  { key: 'partner', label: tr('Partners'), from: tr('Partner of'), to: tr('Partner of') },
  { key: 'friend', label: tr('Friends'), from: tr('Friend of'), to: tr('Friend of') },
  { key: 'ally', label: tr('Allies'), from: tr('Ally of'), to: tr('Ally of') },
  { key: 'rival', label: tr('Rivals'), from: tr('Rival of'), to: tr('Rival of') },
  { key: 'enemy', label: tr('Enemies'), from: tr('Enemy of'), to: tr('Enemy of') },
  { key: 'mentor', label: tr('Mentor / student'), from: tr('Mentor of'), to: tr('Student of') },
  { key: 'custom', label: tr('Something else…') },
]

export const relationKind = (key) => RELATION_KINDS.find((k) => k.key === key) || null

// The relation as character `me` sees it: who the other person is, and the
// phrase that goes before their name ("Child of" Bram).
export function relationFor(rel, me) {
  const mine = rel.from_id === Number(me)
  const other = mine
    ? { id: rel.to_id, name: rel.to_name, picture_path: rel.to_picture }
    : { id: rel.from_id, name: rel.from_name, picture_path: rel.from_picture }
  const kind = relationKind(rel.kind)
  let wording
  if (kind && kind.key !== 'custom') wording = t(mine ? kind.from : kind.to)
  else wording = mine ? rel.label : rel.reverse_label || rel.label
  return { other, wording }
}
