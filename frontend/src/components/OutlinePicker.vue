<template>
  <div class="field outline-picker">
    <span>{{ $t('Outline in the height chart') }}</span>
    <div class="outline-body">
      <svg class="outline-preview" viewBox="-40 -4 80 128" aria-hidden="true">
        <OutlineFigure :outline="preview" :ground="120" :height="118" />
      </svg>
      <div class="outline-choices">
        <div class="outline-row" role="radiogroup" :aria-label="$t('Build')">
          <button
            v-for="b in OUTLINE_BUILDS"
            :key="b.key"
            type="button"
            role="radio"
            class="outline-option is-build"
            :class="{ 'is-on': build === b.key }"
            :aria-checked="build === b.key"
            @click="set('build', b.key)"
          >
            <svg viewBox="-14 -1 28 42" aria-hidden="true">
              <OutlineFigure :outline="{ ...preview, build: b.key }" :ground="40" :height="40" :line="1" />
            </svg>
            {{ $t(b.label) }}
          </button>
        </div>
        <div class="outline-row" role="radiogroup" :aria-label="$t('Sex')">
          <span class="row-label">{{ $t('Sex') }}</span>
          <button
            v-for="s in [{ key: '', label: tr('From the character') }, ...OUTLINE_SEXES]"
            :key="s.key"
            type="button"
            role="radio"
            class="outline-option"
            :class="{ 'is-on': modelValue.sex === s.key }"
            :aria-checked="modelValue.sex === s.key"
            @click="set('sex', s.key)"
          >
            {{ $t(s.label) }}
          </button>
        </div>
        <div class="outline-row" role="radiogroup" :aria-label="$t('Age')">
          <span class="row-label">{{ $t('Age') }}</span>
          <button
            v-for="a in [{ key: '', label: tr('From the character') }, ...OUTLINE_AGES]"
            :key="a.key"
            type="button"
            role="radio"
            class="outline-option"
            :class="{ 'is-on': modelValue.age === a.key }"
            :aria-checked="modelValue.age === a.key"
            @click="set('age', a.key)"
          >
            {{ $t(a.label) }}
          </button>
        </div>
        <p class="outline-hint">
          {{ $t("What's left as \"From the character\" follows each character's gender and age (under 13 a child, under 18 a teen, from 60 elderly).") }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { tr } from '../i18n'
import { OUTLINE_SEXES, OUTLINE_AGES, OUTLINE_BUILDS } from '../bodyOutlines'
import OutlineFigure from './OutlineFigure.vue'

// A body type's outline: a build, and a sex and age that are either fixed
// or taken from each character ('').
const props = defineProps({ modelValue: { type: Object, required: true } }) // { sex, age, build }
const emit = defineEmits(['update:modelValue'])

const build = computed(() => props.modelValue.build || 'average')
const preview = computed(() => ({
  sex: props.modelValue.sex || 'neutral',
  age: props.modelValue.age || 'adult',
  build: build.value,
}))

function set(part, value) {
  emit('update:modelValue', { ...props.modelValue, [part]: value })
}
</script>

<style scoped>
.outline-body {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
}

.outline-preview {
  width: 5.5rem;
  flex-shrink: 0;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--glass-border);
}

.outline-choices {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
}

.outline-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}

.row-label {
  width: 2.6rem;
  font-size: 0.78rem;
  color: var(--text-faint);
}

.outline-option {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  padding: 0.3rem 0.6rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.78rem;
  cursor: pointer;
}

.outline-option.is-build {
  padding: 0.35rem 0.45rem 0.3rem;
  min-width: 3.6rem;
}

.outline-option.is-build svg {
  width: 2rem;
  height: 3rem;
}

.outline-option:hover {
  color: var(--text-primary);
}

.outline-option.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.outline-hint {
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.45;
  margin: 0.1rem 0 0;
}

@media (max-width: 560px) {
  .outline-body {
    flex-direction: column;
    align-items: center;
  }
}
</style>
