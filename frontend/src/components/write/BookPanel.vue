<template>
  <aside class="side-panel glass-panel panel-solid book-panel" :aria-label="$t('Book and page')">
    <header class="side-head">
      <h2>{{ $t('Book and page') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <div class="type-line">
      <div>
        <span class="label">{{ $t('Page type') }}</span>
        <strong>{{ typeName }}</strong>
      </div>
      <button type="button" class="btn btn-ghost small" @click="$emit('change-type')">{{ $t('Change…') }}</button>
    </div>

    <template v-if="!online">
      <p class="hint">{{ $t('Text block: {w} mm wide.', { w: textWidth }) }}</p>
      <div v-if="settings.page_type === 'custom'" class="grid">
        <label class="field">
          <span>{{ $t('Width (mm)') }}</span>
          <input type="number" min="50" max="500" step="0.1" :value="page.width_mm" @change="setPage('width_mm', $event.target.value, 50, 500)" />
        </label>
        <label class="field">
          <span>{{ $t('Height (mm)') }}</span>
          <input type="number" min="50" max="500" step="0.1" :value="page.height_mm" @change="setPage('height_mm', $event.target.value, 50, 500)" />
        </label>
      </div>
      <h3>{{ $t('Margins (mm)') }}</h3>
      <div class="grid">
        <label class="field">
          <span>{{ $t('Top') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_top_mm" @change="setPage('margin_top_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ $t('Bottom') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_bottom_mm" @change="setPage('margin_bottom_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ page.mirror ? $t('Inside') : $t('Left') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_inner_mm" @change="setPage('margin_inner_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ page.mirror ? $t('Outside') : $t('Right') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_outer_mm" @change="setPage('margin_outer_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ $t('Gutter') }}</span>
          <input type="number" min="0" max="50" step="0.5" :value="page.gutter_mm" @change="setPage('gutter_mm', $event.target.value, 0, 50)" />
        </label>
        <label class="check">
          <input type="checkbox" :checked="!!page.mirror" @change="setPage('mirror', $event.target.checked)" />
          {{ $t('Mirror margins (facing pages)') }}
        </label>
      </div>
      <p class="hint">{{ $t('Headers, footers and page numbers come with the book layout.') }}</p>
    </template>
    <p v-else class="hint">{{ $t('An online post has no pages: each chapter is one post, read at a comfortable width.') }}</p>
  </aside>
</template>

<script setup>
// The book's page: its type (changing it warns first) and, for printed
// books, the size and margins that set the text block's width.
import { computed } from 'vue'
import { t } from '../../i18n'
import { textBlockMm, pageTypeName } from '../../editor/pageTypes.js'

const props = defineProps({
  settings: { type: Object, required: true },
})
const emit = defineEmits(['update', 'change-type', 'close'])

const online = computed(() => props.settings.page_type === 'online')
const page = computed(() => props.settings.page || {})
const textWidth = computed(() => textBlockMm(props.settings))
const typeName = computed(() => (online.value ? t('Online Post') : props.settings.page_type === 'custom' ? `${t('Custom size')} · ${pageTypeName(props.settings)}` : pageTypeName(props.settings)))

function setPage(key, raw, min = 0, max = 100) {
  let v = raw
  if (typeof raw !== 'boolean') {
    v = +raw
    if (raw === '' || !Number.isFinite(v)) return
    v = Math.min(max, Math.max(min, Math.round(v * 10) / 10))
  }
  emit('update', { ...props.settings, page: { ...page.value, [key]: v } })
}
</script>

<style scoped>
.type-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  padding: 0.6rem 0.7rem;
  border-radius: 10px;
  background: var(--glass);
  border: 1px solid var(--glass-border);
}

.label {
  display: block;
  font-size: 0.72rem;
  color: var(--text-faint);
}

h3 {
  margin: 0.9rem 0 0.4rem;
  font-size: 0.95rem;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem 0.6rem;
}

.grid .field {
  margin: 0;
}

.grid .field > span {
  font-size: 0.75rem;
}

.check {
  grid-column: span 2;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.hint {
  margin: 0.6rem 0 0;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
}
</style>
