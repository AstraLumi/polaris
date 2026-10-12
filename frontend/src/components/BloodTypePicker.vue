<template>
  <div class="field blood-picker">
    <span>{{ $t('Blood type') }}</span>
    <div class="blood-panel">
      <div class="blood-buttons" role="radiogroup" :aria-label="$t('Blood type')">
        <button
          v-for="b in choices"
          :key="b.name"
          type="button"
          role="radio"
          class="blood-option"
          :class="{ 'is-on': isOn(b.name) }"
          :aria-checked="isOn(b.name)"
          @click="pick(b.name)"
          @mouseenter="hovered = b"
          @mouseleave="hovered = null"
          @focus="hovered = b"
          @blur="hovered = null"
        >
          {{ b.name }}
        </button>
        <button type="button" class="blood-option is-none" :class="{ 'is-on': !modelValue }" :aria-checked="!modelValue" role="radio" @click="pick('')">
          {{ $t('Not set') }}
        </button>
      </div>
      <p v-if="shownNote" class="blood-note">
        <strong>{{ shown.name }}</strong> {{ shownNote }}
      </p>
      <p v-else-if="!choices.length" class="blood-note">{{ $t('This story has no blood types. Add some in Settings.') }}</p>
      <router-link to="/settings#blood-types" class="blood-settings">{{ $t('Change the blood types') }}</router-link>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { bloodTypes, bloodTypeNote } from '../bloodTypes'

// One button per blood type of the story (Settings → Blood types), with what
// the hovered or chosen one is said to mean.
const props = defineProps({ modelValue: { type: String, default: '' } })
const emit = defineEmits(['update:modelValue'])

const hovered = ref(null)
const same = (a, b) => (a || '').trim().toLowerCase() === (b || '').trim().toLowerCase()
const isOn = (name) => same(name, props.modelValue)

// The story's types, plus the character's own if it isn't one of them (typed
// before the list existed, or since removed from it).
const choices = computed(() => {
  const list = [...bloodTypes.list]
  if (props.modelValue && !list.some((b) => isOn(b.name))) list.push({ name: props.modelValue, note: '' })
  return list
})

const shown = computed(() => hovered.value || choices.value.find((b) => isOn(b.name)) || null)
const shownNote = computed(() => bloodTypeNote(shown.value))

function pick(name) {
  emit('update:modelValue', name)
}
</script>

<style scoped>
.blood-panel {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  padding: 0.75rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
}

.blood-buttons {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(3.6rem, 1fr));
  gap: 0.35rem;
}

.blood-option {
  min-height: var(--control-h);
  padding: 0 0.5rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.9rem;
  font-weight: 700;
  cursor: pointer;
}

.blood-option.is-none {
  grid-column: span 2;
  font-weight: 400;
  font-size: 0.82rem;
}

.blood-option.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.blood-note {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-muted);
}

.blood-note strong {
  color: var(--text-primary);
  margin-right: 0.25rem;
}

.blood-settings {
  align-self: flex-start;
  font-size: 0.74rem;
  color: var(--text-faint);
}
</style>
