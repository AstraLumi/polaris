<template>
  <div class="card glass-panel" :class="{ 'is-selected': selected }">
    <label v-if="selectMode" class="select-box">
      <input type="checkbox" :checked="selected" @change="$emit('toggle-select')" />
    </label>

    <div class="card-row">
      <div class="portrait">
        <img v-if="character.picture_path" :src="character.picture_path" :alt="character.name" />
        <span v-else class="portrait-fallback">{{ initials }}</span>
      </div>
      <div class="identity">
        <p class="name-line">{{ character.name }} <span class="level">{{ $t('Lv.') }} {{ paddedLevel }}</span></p>
        <p v-if="character.nickname" class="nickname">"{{ character.nickname }}"</p>
        <p class="class-line">
          <span v-if="character.class_icon" class="class-icon"><IconImage :src="character.class_icon" :name="character.class_name" /></span>
          {{ classLine }}
        </p>
        <p class="stat-line">{{ character.hp }} HP | {{ character.mp }} MP</p>
      </div>
    </div>

    <div class="card-actions">
      <RouterLink :to="`/characters/${character.id}`" class="btn btn-ghost small">{{ $t('View') }}</RouterLink>
      <RouterLink :to="`/characters/${character.id}/versions`" class="btn btn-ghost small">{{ $t('Edit') }}</RouterLink>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { t } from '../i18n'
import IconImage from './IconImage.vue'

const props = defineProps({
  character: { type: Object, required: true },
  selectMode: { type: Boolean, default: false },
  selected: { type: Boolean, default: false },
})
defineEmits(['toggle-select'])

const paddedLevel = computed(() => String(props.character.level).padStart(2, '0'))

const classLine = computed(() => {
  const { class_name, subclass_name } = props.character
  if (class_name && subclass_name) return `${class_name} | ${subclass_name}`
  return class_name || subclass_name || t('Unclassed')
})

const initials = computed(() =>
  props.character.name
    .split(' ')
    .filter(Boolean)
    .map((p) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
)
</script>

<style scoped>
.card {
  padding: 1.1rem;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  position: relative;
  border: 1px solid var(--glass-border);
}

.card.is-selected {
  border-color: var(--accent);
}

.select-box {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
}

.card-row {
  display: flex;
  gap: 0.85rem;
}

.portrait {
  width: 64px;
  height: 64px;
  border-radius: 12px;
  background: var(--accent-soft);
  overflow: hidden;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.portrait img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.portrait-fallback {
  font-family: 'Fraunces', serif;
  color: var(--accent);
  font-size: 1.1rem;
}

.identity {
  min-width: 0;
}

.name-line {
  margin: 0;
  font-family: 'Fraunces', serif;
  font-size: 1.05rem;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.level {
  font-size: 0.75rem;
  color: var(--text-faint);
  font-family: 'Manrope', sans-serif;
}

.nickname {
  margin: 0.15rem 0 0;
  font-size: 0.82rem;
  color: var(--text-muted);
  font-style: italic;
}

.class-line,
.stat-line {
  margin: 0.3rem 0 0;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.card-actions {
  display: flex;
  gap: 0.6rem;
}
</style>
