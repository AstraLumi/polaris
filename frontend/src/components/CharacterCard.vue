<template>
  <div class="card glass-panel" :class="{ 'is-selected': selected, 'is-dead': character.status === 'dead' }">
    <!-- The whole card opens the sheet (or, while picking, ticks the box):
         the link's or the box's area is stretched over the card, under its
         buttons, so nothing is nested inside a link. -->
    <label v-if="selectMode" class="select-box">
      <input type="checkbox" :checked="selected" :aria-label="character.name" @change="$emit('toggle-select')" />
    </label>

    <div class="card-row">
      <div class="portrait">
        <img v-if="character.picture_path" :src="character.picture_path" :alt="character.name" />
        <span v-else class="portrait-fallback">{{ initials }}</span>
      </div>
      <div class="identity">
        <p class="name-line">
          <RouterLink v-if="!selectMode" :to="`/characters/${character.id}`" class="card-link" :title="$t('Open the character sheet')">{{ character.name }}</RouterLink>
          <template v-else>{{ character.name }}</template>{{ ' ' }}<span class="level">{{ $t('Lv.') }} {{ paddedLevel }}</span>
        </p>
        <p v-if="character.status && character.status !== 'alive'" class="status-line">
          <span class="status-badge" :class="'is-' + character.status">{{ statusLabel(character.status) }}</span>
        </p>
        <p v-if="character.nickname" class="nickname">"{{ character.nickname }}"</p>
        <p v-if="classLine" class="class-line">
          <span v-if="character.class_icon" class="class-icon"><IconImage :src="character.class_icon" :name="character.class_name" /></span>
          {{ classLine }}
        </p>
        <p v-if="character.subclass_name" class="class-line">{{ character.subclass_name }}</p>
        <p v-if="character.stats?.length" class="stat-line">{{ character.stats.map((s) => `${Math.round(s.value)} ${$t(s.name)}`).join(" | ") }}</p>
      </div>
    </div>

    <div v-if="!selectMode" class="card-actions">
      <RouterLink :to="`/characters/${character.id}/versions/${character.version_id}/edit`" class="btn btn-ghost small" :title="$t('Edit the current version')">{{ $t('Edit') }}</RouterLink>
      <RouterLink :to="`/characters/${character.id}/versions`" class="btn btn-ghost small" :title="$t('Every version of this character, and what they all share')">{{ $t('Versions') }}</RouterLink>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { t } from '../i18n'
import IconImage from './IconImage.vue'
import { statusLabel } from '../characterStatus'

const props = defineProps({
  character: { type: Object, required: true },
  selectMode: { type: Boolean, default: false },
  selected: { type: Boolean, default: false },
})
defineEmits(['toggle-select'])

const paddedLevel = computed(() => String(props.character.level).padStart(2, '0'))

// Class | Specialization; the subclass has a line of its own.
const classLine = computed(() => {
  const { class_name, specialization_name, subclass_name } = props.character
  const parts = [class_name, specialization_name].filter(Boolean)
  if (parts.length) return parts.join(' | ')
  return subclass_name ? '' : t('Unclassed')
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
.status-line {
  margin: 0.2rem 0 0.1rem;
}

/* The dead stay on the list, a little faded. */
.card.is-dead .portrait {
  filter: grayscale(0.85);
  opacity: 0.7;
}

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

.card:hover,
.card:has(.card-link:focus-visible) {
  border-color: color-mix(in srgb, var(--accent) 45%, var(--glass-border));
}

/* The link's (or the box's) area covers the whole card; the buttons sit
   above it. */
.card-link {
  color: inherit;
  text-decoration: none;
  outline: none;
}

.card-link::after,
.select-box {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 1;
  cursor: pointer;
}

.select-box {
  display: flex;
  justify-content: flex-end;
  padding: 0.75rem;
}

.select-box input {
  height: fit-content;
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
  position: relative;
  z-index: 2;
  display: flex;
  gap: 0.6rem;
}
</style>
