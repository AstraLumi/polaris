<template>
  <div class="overlay" @click.self="cancel">
    <div class="modal glass-panel panel-solid is-wide page-type-dialog" role="dialog" aria-modal="true" :aria-label="title">
      <h2>{{ title }}</h2>
      <p class="lead">
        {{ switching
          ? $t('Formatting may not carry over: the whole book takes the new type\'s page and built-in styles. Every chapter is saved as a version first.')
          : $t('The whole book is formatted for one page type. You can change it later.') }}
      </p>

      <div class="types" role="radiogroup" :aria-label="$t('Page type')">
        <button
          v-for="k in PAGE_TYPE_KEYS"
          :key="k"
          type="button"
          class="type"
          role="radio"
          :aria-checked="picked === k"
          :class="{ 'is-picked': picked === k, 'is-current': switching && current?.page_type === k }"
          @click="picked = k"
        >
          <span class="sheet-icon" :class="`is-${k}`" aria-hidden="true"><span></span></span>
          <span class="type-name">{{ $t(PAGE_TYPE_LABELS[k]) }}</span>
          <span class="type-sub">{{ subOf(k) }}</span>
        </button>
      </div>

      <div v-if="picked === 'custom'" class="custom">
        <label class="field">
          <span>{{ $t('Width (mm)') }}</span>
          <input v-model.number="width" type="number" min="50" max="1000" step="0.1" />
        </label>
        <span class="times" aria-hidden="true">×</span>
        <label class="field">
          <span>{{ $t('Height (mm)') }}</span>
          <input v-model.number="height" type="number" min="50" max="1000" step="0.1" />
        </label>
        <div class="presets">
          <button v-for="p in PRESETS" :key="p.label" type="button" class="btn btn-ghost small" @click="width = p.w; height = p.h">{{ p.label }}</button>
        </div>
      </div>
      <p v-if="picked === 'custom' && !sizeOk" class="error-banner">{{ $t('Width and height must be between 50 and 1000 mm.') }}</p>

      <div class="modal-actions">
        <button v-if="!switching" type="button" class="btn btn-ghost import-btn" @click="$emit('import')">{{ $t('Upload a .docx instead') }}</button>
        <button type="button" class="btn btn-ghost" @click="cancel">{{ switching ? $t('Cancel') : $t('Later') }}</button>
        <button type="button" class="btn btn-primary" :disabled="!picked || !sizeOk || busy" @click="choose">
          {{ switching ? $t('Switch page type') : $t('Use this page type') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
// Picks a book's page type: on a first visit to a book never set up, and
// when switching (which warns first).
import { ref, computed } from 'vue'
import { t } from '../../i18n'
import { PAGE_TYPE_KEYS, PAGE_TYPE_LABELS, PAGE_TYPES } from '../../editor/pageTypes.js'

const props = defineProps({
  current: { type: Object, default: null }, // the book's settings
  switching: { type: Boolean, default: false },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['choose', 'cancel', 'import'])

const PRESETS = [
  { label: '5 × 8 in', w: 127, h: 203.2 },
  { label: '5.5 × 8.5 in', w: 139.7, h: 215.9 },
  { label: '6 × 9 in', w: 152.4, h: 228.6 },
  { label: '110 × 178 mm', w: 110, h: 178 },
]

const picked = ref(props.switching ? '' : props.current?.page_type && props.current.page_type !== 'a5' ? props.current.page_type : '')
const width = ref(props.current?.page_type === 'custom' ? props.current.page?.width_mm : PAGE_TYPES.custom.width_mm)
const height = ref(props.current?.page_type === 'custom' ? props.current.page?.height_mm : PAGE_TYPES.custom.height_mm)

const title = computed(() => (props.switching ? t('Change the page type') : t('What is this book for?')))
const sizeOk = computed(() => picked.value !== 'custom' || [width.value, height.value].every((v) => +v >= 50 && +v <= 1000))

function subOf(k) {
  const p = PAGE_TYPES[k]
  if (k === 'online') return t('No pages: one chapter per post, a blank line between paragraphs')
  if (k === 'custom') return t('Width × height in mm; body text {pt} pt', { pt: p.size })
  return t('{w} × {h} mm; body text {pt} pt', { w: p.width_mm, h: p.height_mm, pt: p.size })
}

function choose() {
  if (!picked.value || !sizeOk.value) return
  emit('choose', { type: picked.value, width_mm: +width.value, height_mm: +height.value })
}
const cancel = () => emit('cancel')
</script>

<style scoped>
.overlay {
  z-index: 80;
}

.lead {
  margin: -0.5rem 0 1.1rem;
  color: var(--text-muted);
  font-size: 0.9rem;
  line-height: 1.5;
}

.types {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(9.5rem, 1fr));
  gap: 0.6rem;
}

.type {
  display: grid;
  justify-items: center;
  gap: 0.3rem;
  padding: 0.9rem 0.6rem 0.8rem;
  border-radius: 12px;
  border: 1px solid var(--glass-border);
  background: var(--glass);
  color: var(--text-primary);
  font: inherit;
  cursor: pointer;
  text-align: center;
}

.type:hover {
  border-color: var(--accent);
}

.type.is-picked {
  border-color: var(--accent);
  background: var(--accent-soft);
  box-shadow: 0 0 0 1px var(--accent);
}

.type.is-current .type-name::after {
  content: ' •';
  color: var(--accent);
}

.type-name {
  font-weight: 700;
}

.type-sub {
  font-size: 0.74rem;
  color: var(--text-muted);
  line-height: 1.35;
}

/* A little sheet drawn to each page's proportions. */
.sheet-icon {
  display: grid;
  place-items: center;
  height: 3.4rem;
}

.sheet-icon span {
  display: block;
  background: #f6f2e8;
  border-radius: 2px;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.35);
}

.sheet-icon.is-a4 span { width: 2.3rem; height: 3.25rem; }
.sheet-icon.is-a5 span { width: 1.95rem; height: 2.75rem; }
.sheet-icon.is-a6 span { width: 1.6rem; height: 2.25rem; }
.sheet-icon.is-custom span { width: 1.85rem; height: 2.8rem; border: 1px dashed #999; }
.sheet-icon.is-online span { width: 3rem; height: 2.1rem; border-radius: 4px; background: linear-gradient(#f6f2e8 0 22%, #e2ddd0 22% 26%, #f6f2e8 26%); }

.custom {
  display: flex;
  align-items: flex-end;
  gap: 0.6rem;
  flex-wrap: wrap;
  margin-top: 1rem;
}

.custom .field {
  width: 8rem;
  margin: 0;
}

.times {
  padding-bottom: 0.6rem;
  color: var(--text-muted);
}

.import-btn {
  margin-right: auto;
}

.presets {
  display: flex;
  gap: 0.3rem;
  flex-wrap: wrap;
}
</style>
