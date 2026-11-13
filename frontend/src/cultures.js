// Cultures (backend/cultures.go): the kinds of element a culture has, and
// the filters its pages and the reference library use.
import { tr } from './i18n'

// In the order the culture page shows them; the same keys as
// cultureElementKinds in backend/cultures.go.
export const ELEMENT_KINDS = [
  { id: 'clothing', label: tr('Clothing') },
  { id: 'food', label: tr('Food') },
  { id: 'festival', label: tr('Festivals') },
  { id: 'custom', label: tr('Customs') },
  { id: 'belief', label: tr('Beliefs') },
  { id: 'art', label: tr('Arts') },
  { id: 'architecture', label: tr('Architecture') },
  { id: 'language', label: tr('Language') },
  { id: 'other', label: tr('Other') },
]

// Messages the server can send back; listed so they get translated.
void [
  tr('culture not found'), tr('element not found'), tr('name is required'), tr('a culture with that name already exists'),
  tr('invalid kingdom list'), tr('invalid list'), tr('failed to save the culture'), tr('failed to delete the culture'),
  tr('failed to load the cultures'), tr('failed to save the element'), tr('failed to delete the element'),
  tr('an element holds at most 40 pictures'), tr('a reference holds at most 40 pictures'),
]

// The pure filters live apart, so node tests can load them.
export * from './cultureFilters.js'
