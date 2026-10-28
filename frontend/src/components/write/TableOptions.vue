<template>
  <div ref="root" class="table-options" :style="place" @mousedown.stop>
    <button
      type="button"
      class="chip glass-panel panel-solid"
      :class="{ 'is-open': open }"
      :title="$t('Table options')"
      :aria-label="$t('Table options')"
      :aria-expanded="open"
      @mousedown.prevent
      @click="open = !open"
    >
      <Icon d="M3 5h18v14H3zM3 10h18M3 14.5h18M9 5v14M15 5v14" />
    </button>
    <div v-if="open" class="panel glass-panel panel-solid" role="group" :aria-label="$t('Table options')">
    <div class="row">
      <div class="group" role="group" :aria-label="$t('Rows')">
        <button v-bind="btn(t('Insert a row above'))" @click="run(addRowBefore)"><Icon d="M3 13h18v7H3zM3 16.5h18M12 3v7M9 6l3-3 3 3" /></button>
        <button v-bind="btn(t('Insert a row below'))" @click="run(addRowAfter)"><Icon d="M3 4h18v7H3zM3 7.5h18M12 14v7M9 18l3 3 3-3" /></button>
        <button v-bind="btn(t('Delete the row'))" :disabled="info.rows < 2" @click="run(deleteRow)"><Icon d="M3 8h18v8H3zM9 12h6" /></button>
      </div>
      <div class="group" role="group" :aria-label="$t('Columns')">
        <button v-bind="btn(t('Insert a column to the left'))" :disabled="info.cols >= MAX_TABLE_COLS" @click="run(addColumnBefore)"><Icon d="M13 3h7v18h-7zM16.5 3v18M3 12h7M6 9l-3 3 3 3" /></button>
        <button v-bind="btn(t('Insert a column to the right'))" :disabled="info.cols >= MAX_TABLE_COLS" @click="run(addColumnAfter)"><Icon d="M4 3h7v18H4zM7.5 3v18M14 12h7M18 9l3 3-3 3" /></button>
        <button v-bind="btn(t('Delete the column'))" :disabled="info.cols < 2" @click="run(deleteColumn)"><Icon d="M8 3h8v18H8zM12 9v6" /></button>
      </div>
      <div class="group">
        <button v-bind="btn(t('Merge cells'))" :disabled="!info.canMerge" @click="run(mergeCells)"><Icon d="M3 5h18v14H3zM7 12h10M14 9.5l2.5 2.5-2.5 2.5M10 9.5 7.5 12l2.5 2.5" /></button>
        <button v-bind="btn(t('Split the cell'))" :disabled="!info.canSplit" @click="run(splitCell)"><Icon d="M3 5h18v14H3zM12 5v14" /></button>
        <button v-bind="btn(t('Header row: repeats on every page'), info.headerRow)" @click="run(toggleHeaderRow)"><Icon d="M3 5h18v14H3zM3 10h18M7 7.5h10" /></button>
      </div>
    </div>
    <div class="row">
      <div class="group shades" role="group" :aria-label="$t('Cell shading')">
        <button
          v-for="c in SHADES"
          :key="c"
          type="button"
          class="sw"
          :class="{ 'is-active': cell.background === c }"
          :style="{ background: c }"
          :title="$t('Cell shading') + ' ' + c"
          :aria-label="$t('Cell shading') + ' ' + c"
          @mousedown.prevent
          @click="run(setCellsAttr('background', c))"
        ></button>
        <label class="other" :title="$t('Other colour')">
          <input type="color" :value="cell.background || '#d9d9d9'" :aria-label="$t('Other colour')" @change="run(setCellsAttr('background', $event.target.value))" />
        </label>
        <button v-bind="btn(t('No shading'), false)" class="none" @click="run(setCellsAttr('background', null))"><Icon d="M5 5l14 14M4 4h16v16H4z" /></button>
      </div>
      <div class="group" role="group" :aria-label="$t('Vertical alignment')">
        <button v-for="v in VALIGNS" :key="v.key" v-bind="btn(v.label, (cell.valign || 'top') === v.key)" @click="run(setCellsAttr('valign', v.key === 'top' ? null : v.key))"><Icon :d="v.icon" /></button>
      </div>
    </div>
    <div class="row">
      <div class="group" role="group" :aria-label="$t('Table alignment')">
        <button v-for="a in ALIGNS" :key="a.key" v-bind="btn(a.label, (info.attrs.align || 'left') === a.key)" @click="run(setTableAttrs({ align: a.key === 'left' ? null : a.key }))"><Icon :d="a.icon" /></button>
      </div>
      <label class="pick">
        <span>{{ $t('Width') }}</span>
        <select :value="info.attrs.width === 'full' ? 'full' : 'auto'" :aria-label="$t('Width')" @change="run(setTableAttrs({ width: $event.target.value === 'full' ? 'full' : null }))">
          <option value="auto">{{ $t('Fit the columns') }}</option>
          <option value="full">{{ $t('Full width') }}</option>
          <option v-if="typeof info.attrs.width === 'number'" value="fixed" disabled>{{ $t('{n} pt', { n: Math.round(info.attrs.width * 0.75) }) }}</option>
        </select>
      </label>
      <label class="pick">
        <span>{{ $t('Borders') }}</span>
        <select :value="info.attrs.borders || 'all'" :aria-label="$t('Borders')" @change="run(setTableAttrs({ borders: $event.target.value }))">
          <option value="all">{{ $t('All borders') }}</option>
          <option value="outer">{{ $t('Outside border') }}</option>
          <option value="none">{{ $t('No borders') }}</option>
        </select>
      </label>
      <button type="button" class="btn btn-danger small delete" @mousedown.prevent @click="run(deleteTable)">{{ $t('Delete table') }}</button>
    </div>
    </div>
  </div>
</template>

<script setup>
// What can be done to the table the cursor is in (docs/EDITOR.md,
// "Tables"): rows and columns, merging, the header row, the cells'
// shading and vertical alignment, the table's alignment, width and
// borders. A small button sits by the table's top right corner (in the
// page's margin when there is room), so nothing covers the text; it opens
// the options below it, which close on a click anywhere else or Esc.
// `info` is tableInfo() of the editor's state and `exec` runs a command on
// the editor; top/left place the button.
import { ref, computed, h, onMounted, onBeforeUnmount } from 'vue'
import { t } from '../../i18n'
import {
  addRowBefore,
  addRowAfter,
  deleteRow,
  addColumnBefore,
  addColumnAfter,
  deleteColumn,
  mergeCells,
  splitCell,
  toggleHeaderRow,
  deleteTable,
  setTableAttrs,
  setCellsAttr,
} from '../../editor/tables.js'
import { MAX_TABLE_COLS } from '../../editor/tableSchema.js'

const props = defineProps({
  info: { type: Object, required: true },
  exec: { type: Function, required: true },
  top: { type: Number, default: 0 },
  left: { type: Number, default: 0 },
})

const root = ref(null)
const open = ref(false)
defineExpose({ root })

const close = () => (open.value = false)
const onKey = (e) => {
  if (e.key === 'Escape' && open.value) {
    e.stopPropagation()
    close()
  }
}
onMounted(() => {
  document.addEventListener('mousedown', close)
  document.addEventListener('keydown', onKey, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', close)
  document.removeEventListener('keydown', onKey, true)
})

// Light shades that keep black text readable (stat blocks, status screens).
const SHADES = ['#f2f2f2', '#d9d9d9', '#deeaf6', '#e2efd9', '#fff2cc', '#fbe4d5', '#e4dfec']
const ALIGNS = computed(() => [
  { key: 'left', label: t('Align left'), icon: 'M3 6h18M3 10h12M3 14h18M3 18h12' },
  { key: 'center', label: t('Centre'), icon: 'M3 6h18M6 10h12M3 14h18M6 18h12' },
  { key: 'right', label: t('Align right'), icon: 'M3 6h18M9 10h12M3 14h18M9 18h12' },
])
const VALIGNS = computed(() => [
  { key: 'top', label: t('Text at the top of the cell'), icon: 'M4 4h16v16H4zM8 8h8M8 11h8' },
  { key: 'middle', label: t('Text in the middle of the cell'), icon: 'M4 4h16v16H4zM8 10.5h8M8 13.5h8' },
  { key: 'bottom', label: t('Text at the bottom of the cell'), icon: 'M4 4h16v16H4zM8 13h8M8 16h8' },
])

const cell = computed(() => props.info.cell || {})
const place = computed(() => ({ top: `${props.top}px`, left: `${props.left}px` }))
const run = (cmd) => props.exec(cmd)

// Buttons keep the text's selection: they never take the focus on click.
function btn(label, active) {
  return {
    type: 'button',
    class: ['tb-btn', { 'is-active': !!active }],
    title: label,
    'aria-label': label,
    'aria-pressed': active === undefined ? undefined : !!active,
    onMousedown: (e) => e.preventDefault(),
  }
}

const Icon = (p) =>
  h('svg', { viewBox: '0 0 24 24', width: 16, height: 16, fill: 'none', stroke: 'currentColor', 'stroke-width': 1.8, 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true' }, [h('path', { d: p.d })])
Icon.props = ['d']
</script>

<style scoped>
.table-options {
  position: absolute;
  z-index: 20;
  font-size: 0.8rem;
}

.chip {
  display: grid;
  place-items: center;
  width: 1.75rem;
  height: 1.75rem;
  padding: 0;
  border-radius: 8px;
  color: var(--text-muted);
  cursor: pointer;
}

.chip:hover,
.chip.is-open {
  color: var(--accent);
}

.panel {
  position: absolute;
  top: calc(100% + 0.3rem);
  right: 0;
  display: grid;
  gap: 0.4rem;
  width: min(27rem, calc(100vw - 2rem));
  padding: 0.5rem 0.6rem;
  border-radius: 12px;
}

.row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.3rem 0.6rem;
}

.group {
  display: flex;
  align-items: center;
  gap: 0.1rem;
}

.tb-btn {
  display: inline-grid;
  place-items: center;
  width: 1.9rem;
  height: 1.9rem;
  border: 1px solid transparent;
  border-radius: 7px;
  background: none;
  color: var(--text-primary);
  cursor: pointer;
}

.tb-btn:hover:not(:disabled) {
  background: var(--glass);
  border-color: var(--glass-border);
}

.tb-btn.is-active {
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
  color: var(--accent);
}

.tb-btn:disabled {
  opacity: 0.38;
  cursor: default;
}

.shades {
  gap: 0.2rem;
}

.sw {
  width: 1.35rem;
  height: 1.35rem;
  border-radius: 5px;
  border: 1px solid rgba(127, 127, 127, 0.5);
  cursor: pointer;
}

.sw.is-active {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

.other input {
  width: 1.6rem;
  height: 1.45rem;
  min-height: 0;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
}

.pick {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.pick select {
  min-height: 1.9rem;
  height: 1.9rem;
  width: auto;
  padding: 0.1rem 1.6rem 0.1rem 0.45rem;
  font-size: 0.78rem;
}

.delete {
  margin-left: auto;
}
</style>
