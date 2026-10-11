<template>
  <div class="field">
    <span>{{ $t('Gender') }}</span>
    <div class="gender-picker">
      <div class="gender-icons" role="radiogroup" :aria-label="$t('Gender')">
        <button
          v-for="g in GENDERS"
          :key="g.key"
          type="button"
          role="radio"
          class="gender-option"
          :class="{ 'is-on': kind === g.key }"
          :aria-checked="kind === g.key"
          :aria-label="$t(g.label)"
          @click="emit('update:modelValue', g.key)"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <template v-if="g.key === ''">
              <path d="M9.2 9a3 3 0 1 1 4.3 2.7c-.9.4-1.5 1.2-1.5 2.2v.6" />
              <circle cx="12" cy="18" r=".6" />
            </template>
            <template v-else-if="g.key === 'Male'">
              <circle cx="10" cy="14" r="5" />
              <path d="M13.6 10.4 19 5M14.5 5H19v4.5" />
            </template>
            <template v-else>
              <circle cx="12" cy="9" r="5" />
              <path d="M12 14v7M9 18h6" />
            </template>
          </svg>
        </button>
      </div>
      <input
        :value="kind === null ? modelValue : ''"
        type="text"
        maxlength="60"
        :placeholder="$t('Other…')"
        :aria-label="$t('Other gender')"
        @input="emit('update:modelValue', $event.target.value)"
      />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { GENDERS, genderKind } from '../gender'

// Unknown / male / female as icons, and a box for anything else.
const props = defineProps({ modelValue: { type: String, default: '' } })
const emit = defineEmits(['update:modelValue'])

const kind = computed(() => genderKind(props.modelValue))
</script>

<style scoped>
.gender-picker {
  display: flex;
  gap: 0.35rem;
}

.gender-icons {
  display: flex;
  gap: 0.35rem;
}

.gender-option {
  display: grid;
  place-items: center;
  width: var(--control-h);
  min-height: var(--control-h);
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  cursor: pointer;
}

.gender-option svg {
  width: 1.15rem;
  height: 1.15rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.gender-option:hover {
  color: var(--text-primary);
}

.gender-option.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.gender-picker input {
  flex: 1;
  min-width: 0;
}
</style>
