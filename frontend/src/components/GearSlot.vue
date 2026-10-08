<template>
  <component
    :is="editable ? 'button' : 'div'"
    class="slot"
    :class="{ 'is-filled': !!gear, 'is-editable': editable }"
    :type="editable ? 'button' : undefined"
    :title="tip"
    @click="editable && $emit('open')"
  >
    <span class="slot-art">
      <IconImage :src="gear ? gearIconSrc(gear) : BUILTIN_PREFIX + typeIcon" :name="gear ? gear.name : ''" :alt="gear ? gear.name : ''" />
    </span>
    <span class="slot-text">
      <span class="slot-label">{{ $t(slotDef.label) }}</span>
      <span v-if="gear" class="slot-name">{{ gear.name }}</span>
      <span v-else class="slot-empty">{{ editable ? $t('Tap to equip') : $t('Empty') }}</span>
      <span v-if="gear" class="slot-meta">{{ formatKg(gear.weight) }}<template v-if="summary"> · {{ summary }}</template></span>
    </span>
  </component>
</template>

<script setup>
import { computed } from 'vue'
import { BUILTIN_PREFIX } from '../builtinIcons'
import { gearIconSrc, slotTypeDef, formatBonuses, formatKg } from '../gear'
import IconImage from './IconImage.vue'

const props = defineProps({
  slotDef: { type: Object, required: true },
  gear: { type: Object, default: null },
  editable: { type: Boolean, default: false },
})
defineEmits(['open'])

const typeIcon = computed(() => slotTypeDef(props.slotDef.type)?.icon || '')
const bonuses = computed(() => (props.gear ? formatBonuses(props.gear.modifiers) : []))
const summary = computed(() => {
  const b = bonuses.value
  if (!b.length) return ''
  return b.length > 2 ? `${b[0].text}, ${b[1].text} +${b.length - 2}` : b.map((x) => x.text).join(', ')
})
const tip = computed(() => (props.gear ? [props.gear.name, ...bonuses.value.map((b) => b.text)].join('\n') : ''))
</script>

<style scoped>
.slot {
  display: flex; align-items: center; gap: 0.7rem; width: 100%; min-width: 0; box-sizing: border-box; min-height: 62px; padding: 0.55rem 0.7rem;
  font: inherit; color: inherit; text-align: left;
  background: rgba(255,255,255,0.03); border: 1px dashed var(--glass-border); border-radius: 14px;
}
.slot.is-filled { border-style: solid; background: color-mix(in srgb, var(--accent) 8%, transparent); border-color: color-mix(in srgb, var(--accent) 35%, transparent); }
.slot.is-editable { cursor: pointer; }
.slot.is-editable:hover, .slot.is-editable:focus-visible { border-color: var(--accent); }
.slot-art { width: 42px; height: 42px; flex-shrink: 0; border-radius: 10px; background: var(--accent-soft); display: flex; align-items: center; justify-content: center; overflow: hidden; }
.slot:not(.is-filled) .slot-art { opacity: 0.4; }
.slot-text { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.slot-label { font-size: 0.68rem; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-faint); }
.slot-name { font-family: 'Fraunces', serif; font-size: 0.95rem; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.slot-empty { font-size: 0.78rem; color: var(--text-faint); font-style: italic; }
.slot-meta { font-size: 0.72rem; color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>

<style scoped>
@media (max-width: 560px) {
  .slot { padding: 0.45rem; gap: 0.45rem; }
  .slot-art { width: 34px; height: 34px; }
}
</style>
