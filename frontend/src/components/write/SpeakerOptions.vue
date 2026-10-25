<template>
  <div ref="root" class="speaker-options glass-panel panel-solid" role="dialog" :aria-label="$t('Who speaks')" :style="place" @mousedown.stop @keydown.esc.stop.prevent="$emit('close')">
    <label class="field">
      <span>{{ $t('Character') }}</span>
      <select ref="pickEl" :value="attrs.character ?? ''" @change="pick($event.target.value)">
        <option value="">{{ $t('Not one of the characters') }}</option>
        <option v-if="attrs.character != null && !characters.some((c) => c.id === attrs.character)" :value="attrs.character">{{ attrs.speaker || '—' }}</option>
        <option v-for="c in characters" :key="c.id" :value="c.id">{{ c.name }}</option>
      </select>
    </label>
    <div class="row">
      <label class="field">
        <span>{{ $t('Speaker') }}</span>
        <input type="text" maxlength="200" :value="attrs.speaker || ''" :placeholder="$t('Name as shown')" @change="setSpeaker($event.target.value)" @keydown.enter.prevent="$event.target.blur()" />
      </label>
      <label class="field">
        <span>{{ $t('Manner') }}</span>
        <input type="text" maxlength="200" :value="attrs.manner || ''" :placeholder="$t('softly, from the door…')" @change="$emit('change', { manner: $event.target.value.trim() })" @keydown.enter.prevent="$event.target.blur()" />
      </label>
    </div>
    <div class="row end">
      <button type="button" class="btn btn-ghost small" :title="$t('Turn it back into a paragraph “Name, manner: line”')" @click="$emit('unwrap')">{{ $t('Back to plain text') }}</button>
      <button type="button" class="btn btn-primary small" @click="$emit('close')">{{ $t('Done') }}</button>
    </div>
  </div>
</template>

<script setup>
// A dialogue's speaker, character and manner (Script Mode, docs/EDITOR.md
// stage 4), opened by a click on the speaker. `change` is told what to set;
// the page runs the command.
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  attrs: { type: Object, required: true },
  characters: { type: Array, default: () => [] }, // [{ id, name }], by name
  resolve: { type: Function, default: () => null }, // speaker name -> { id, name } | null
  top: { type: Number, default: 0 },
  left: { type: Number, default: 0 },
})
const emit = defineEmits(['change', 'unwrap', 'close'])

const root = ref(null)
const pickEl = ref(null)
defineExpose({ root })
onMounted(() => pickEl.value?.focus())

const place = computed(() => ({ top: `${props.top}px`, left: `${props.left}px` }))

// Picking a character names the speaker after them (unless the speaker is
// one of their names already, a nickname say).
function pick(value) {
  const id = value === '' ? null : +value
  const c = props.characters.find((x) => x.id === id)
  const patch = { character: id }
  const known = props.resolve(props.attrs.speaker)
  if (c && (!props.attrs.speaker || known?.id !== c.id)) patch.speaker = c.name
  emit('change', patch)
}

// A typed speaker who is a character becomes that character; any other
// name is only how the speaker is shown (the character stays).
function setSpeaker(value) {
  const speaker = value.trim()
  const who = props.resolve(speaker)
  emit('change', who ? { speaker, character: who.id } : { speaker })
}
</script>

<style scoped>
.speaker-options {
  position: absolute;
  z-index: 20;
  display: grid;
  gap: 0.5rem;
  width: min(24rem, calc(100% - 1rem));
  padding: 0.6rem 0.7rem;
  border-radius: 12px;
  font-size: 0.82rem;
}

.row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.row > .field {
  flex: 1 1 9rem;
}

.row.end {
  justify-content: flex-end;
}

.field {
  display: grid;
  gap: 0.15rem;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.field input,
.field select {
  min-height: 2rem;
  height: 2rem;
  padding: 0.15rem 0.45rem;
  font-size: 0.82rem;
}
</style>
