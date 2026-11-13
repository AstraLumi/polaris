<template>
  <div class="ref-filters">
    <div class="row">
      <input :value="search" type="search" :placeholder="$t('Search titles, notes and tags…')" :aria-label="$t('Search')" @input="$emit('update:search', $event.target.value)" />
      <div v-if="kinds.length" class="chips" role="radiogroup" :aria-label="$t('Kind')">
        <button
          v-for="k in [{ id: '', label: tr('All') }, ...kinds]"
          :key="k.id"
          type="button"
          role="radio"
          class="chip"
          :class="{ 'is-on': kind === k.id }"
          :aria-checked="kind === k.id"
          @click="$emit('update:kind', k.id)"
        >
          {{ $t(k.label) }}<template v-if="counts && k.id"> · {{ counts[k.id] || 0 }}</template>
        </button>
      </div>
    </div>
    <div v-if="allTags.length" class="chips tags" :aria-label="$t('Tags')">
      <button
        v-for="tg in allTags"
        :key="tg"
        type="button"
        class="chip tag"
        :class="{ 'is-on': isOn(tg) }"
        :aria-pressed="isOn(tg)"
        @click="toggle(tg)"
      >
        #{{ tg }}
      </button>
      <button v-if="tags.length" type="button" class="chip clear" @click="$emit('update:tags', [])">{{ $t('Clear tags') }}</button>
    </div>
  </div>
</template>

<script setup>
// The search box, kind chips and tag chips over a list of references or
// culture elements. Picked tags must all be on an item for it to show.
import { tr } from '../i18n'

const props = defineProps({
  search: { type: String, default: '' },
  kind: { type: String, default: '' },
  tags: { type: Array, default: () => [] },
  kinds: { type: Array, default: () => [] }, // [{ id, label }]
  counts: { type: Object, default: null }, // { kind: n }
  allTags: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:search', 'update:kind', 'update:tags'])

const isOn = (tg) => props.tags.some((x) => x.toLowerCase() === tg.toLowerCase())
function toggle(tg) {
  emit('update:tags', isOn(tg) ? props.tags.filter((x) => x.toLowerCase() !== tg.toLowerCase()) : [...props.tags, tg])
}
</script>

<style scoped>
.ref-filters {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.row input {
  flex: 1 1 16rem;
  max-width: 24rem;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.chip {
  min-height: var(--control-h);
  padding: 0 0.85rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(var(--wash), 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}

.chip.tag,
.chip.clear {
  min-height: 0;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  font-size: 0.78rem;
}

.chip.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.chip.clear {
  border-style: dashed;
}
</style>
