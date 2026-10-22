<template>
  <div class="biomes">
    <div class="head">
      <span class="title">{{ $t('Terrain') }}</span>
      <span class="hint">{{ $t('The shape and height of the land') }}</span>
    </div>
    <div class="swatches">
      <button
        v-for="l in LANDFORMS"
        :key="l.key"
        type="button"
        class="swatch"
        :class="{ active: isPicked('relief', l.key) }"
        :title="$t('{name}: about {m} m', { name: $t(l.label), m: l.height.toLocaleString() })"
        :aria-pressed="isPicked('relief', l.key)"
        @click="pick('relief', l.key)"
      >
        <i class="chip" :style="{ background: l.color }"></i>
        <span class="name">{{ $t(l.label) }}</span>
        <span v-if="counts.get('relief:' + l.key)" class="count">{{ counts.get('relief:' + l.key).toLocaleString() }}</span>
      </button>
    </div>
    <label v-if="landform" class="height">
      <span>{{ $t('Ground height (m)') }}</span>
      <input
        type="number"
        step="100"
        min="-500"
        max="9000"
        :value="height ?? ''"
        :placeholder="String(landform.height)"
        @input="$emit('update:height', $event.target.value === '' ? null : Number($event.target.value))"
      />
    </label>

    <div class="head">
      <span class="title">{{ $t('Biomes') }}</span>
      <span class="hint">{{ $t('What covers the land') }}</span>
    </div>
    <section v-for="g in BIOME_GROUPS" :key="g.id" class="group">
      <span class="group-name">{{ $t(g.label) }}</span>
      <div class="swatches">
        <button
          v-for="b in g.biomes"
          :key="b.key"
          type="button"
          class="swatch"
          :class="{ active: isPicked('biome', b.key) }"
          :title="$t(b.label)"
          :aria-pressed="isPicked('biome', b.key)"
          @click="pick('biome', b.key)"
        >
          <i class="chip" :style="{ background: b.color }"></i>
          <span class="name">{{ $t(b.label) }}</span>
          <span v-if="counts.get('biome:' + b.key)" class="count">{{ counts.get('biome:' + b.key).toLocaleString() }}</span>
        </button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { BIOME_GROUPS, LANDFORMS, landformOf } from '../biomes'

// What to paint on the Continents layer (biomes.js), two things apart: the
// terrain (the land's shape and height) and the biome (what covers it), so
// a plateau can be forested and mountains iced. One is picked at a time:
// { kind: 'relief' | 'biome', key }; picking it again lets go. Terrain is
// painted with a ground height of its own, or the landform's usual one.
// counts: 'kind:key' -> hexes already painted with it.
const props = defineProps({
  modelValue: { type: Object, default: null },
  height: { type: Number, default: null },
  counts: { type: Map, default: () => new Map() },
})
const emit = defineEmits(['update:modelValue', 'update:height'])

const isPicked = (kind, key) => props.modelValue?.kind === kind && props.modelValue?.key === key
const pick = (kind, key) => emit('update:modelValue', isPicked(kind, key) ? null : { kind, key })
const landform = computed(() => (props.modelValue?.kind === 'relief' ? landformOf(props.modelValue.key) : null))
</script>

<style scoped>
.biomes {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.5rem;
}

.head + .swatches,
.head + .group {
  margin-top: -0.15rem;
}

.swatches + .head,
.height + .head {
  margin-top: 0.35rem;
}

.title,
.group-name {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.group-name {
  font-weight: 600;
  letter-spacing: 0.02em;
}

.hint {
  font-size: 0.68rem;
  color: var(--text-faint);
}

.group {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.swatches {
  display: flex;
  flex-direction: column;
}

.swatch {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.18rem 0.35rem;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.76rem;
  text-align: left;
  cursor: pointer;
}

.swatch:hover {
  background: var(--glass-hover, rgba(255, 255, 255, 0.05));
}

.swatch.active {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.chip {
  flex: none;
  width: 0.8rem;
  height: 0.8rem;
  border-radius: 3px;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.3);
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.count {
  font-size: 0.66rem;
  color: var(--text-faint);
}

.height {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  font-size: 0.74rem;
  color: var(--text-muted);
}

.height input {
  width: 6rem;
  min-height: 1.7rem;
  padding: 0.1rem 0.35rem;
  font-size: 0.74rem;
}
</style>
