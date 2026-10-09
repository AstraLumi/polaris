<template>
  <aside class="hex-panel glass-panel panel-solid">
    <header class="panel-head">
      <div>
        <h3>{{ $t('Hex') }}</h3>
        <p class="coords">({{ hex.q }}, {{ hex.r }})</p>
      </div>
      <button type="button" class="icon-btn" :title="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <p v-if="error" class="error-banner">{{ error }}</p>

    <section>
      <h4>{{ $t('Major locations here') }}</h4>
      <p v-if="!panel.majors.length" class="muted">
        {{ $t('None. Pick a major location below and paint this hex.') }}
      </p>
      <ul class="mini-list">
        <li v-for="m in panel.majors" :key="m.id">
          <span class="dot" :class="{ none: !m.color }" :style="m.color ? { background: m.color } : {}"></span>
          <span class="mini-name">{{ m.name }}</span>
          <span class="mini-tag">{{ m.color ? $t('Kingdom') : $t('Major') }}</span>
          <button type="button" class="btn btn-ghost small" @click="$emit('edit-major', m)">{{ $t('Edit') }}</button>
        </li>
      </ul>
    </section>

    <section>
      <div class="section-row">
        <h4>{{ $t('Locations on this hex') }}</h4>
        <button v-if="!formOpen" type="button" class="btn btn-primary small" @click="openAdd">{{ $t('Add') }}</button>
      </div>
      <p v-if="!panel.minors.length && !formOpen" class="muted">{{ $t('No locations yet.') }}</p>

      <ul class="mini-list">
        <li v-for="m in panel.minors" :key="m.id" class="minor-item">
          <div class="minor-main">
            <span class="mini-name">
              {{ m.name }}
              <span v-if="m.is_capital" class="badge is-capital">{{ $t('Capital') }}</span>
              <span v-else-if="m.is_city" class="badge">{{ $t('City') }}</span>
            </span>
            <span class="belongs">{{ $t('Belongs to: {name}', { name: belongsLabel(m) }) }}</span>
          </div>
          <div class="minor-actions">
            <button type="button" class="btn btn-ghost small" @click="openEdit(m)">{{ $t('Edit') }}</button>
            <button type="button" class="btn btn-ghost small" @click="confirmTarget = m">{{ $t('Delete') }}</button>
          </div>
        </li>
      </ul>

      <form v-if="formOpen" class="minor-form" @submit.prevent="submit">
        <label class="field">
          <span>{{ $t('Name') }}</span>
          <input v-model="name" type="text" required autofocus />
        </label>
        <div class="settlement">
          <span class="settlement-title">{{ $t('Settlement') }}</span>
          <label class="check"><input v-model="isCity" type="checkbox" /> {{ $t('City') }}</label>
          <label class="check"><input v-model="isCapital" type="checkbox" /> {{ $t('Capital') }}</label>
          <p class="settlement-hint">{{ $t('A city shows its name from further out. A capital is always labelled, whatever the zoom.') }}</p>
        </div>
        <DateField v-model="founding" :label="$t('Founding date')" />
        <label class="field">
          <span>{{ $t('Belongs to') }}</span>
          <select v-model="belongs">
            <option value="">{{ panel.kingdom ? $t('Automatic ({name})', { name: panel.kingdom.name }) : $t('Automatic (no kingdom here)') }}</option>
            <option v-for="m in allMajors" :key="m.id" :value="m.id">{{ m.name }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="desc" rows="3"></textarea>
        </label>
        <div class="form-actions">
          <button type="button" class="btn btn-ghost small" @click="closeForm">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary small" :disabled="saving">
            {{ saving ? $t('Saving…') : editingId ? $t('Save') : $t('Add location') }}
          </button>
        </div>
      </form>
    </section>

    <section v-if="unplaced.length">
      <h4>{{ $t('Unplaced locations') }}</h4>
      <p class="muted">
        {{ $t('Created before the map existed (for example from a spell\'s Origin) and not on a hex yet.') }}
      </p>
      <ul class="mini-list">
        <li v-for="m in unplaced" :key="m.id" class="minor-item">
          <span class="mini-name">{{ m.name }}</span>
          <div class="minor-actions">
            <button type="button" class="btn btn-ghost small" @click="place(m)">{{ $t('Place here') }}</button>
            <button type="button" class="btn btn-ghost small" @click="confirmTarget = m">{{ $t('Delete') }}</button>
          </div>
        </li>
      </ul>
    </section>

    <Teleport to="body">
      <ConfirmDialog
        v-if="confirmTarget"
        :title="$t('Delete {name}?', { name: confirmTarget.name })"
        :message="$t('Spells that use it as their Origin lose that origin.')"
        :confirm-label="$t('Delete')"
        @cancel="confirmTarget = null"
        @confirm="confirmDelete"
      />
    </Teleport>
  </aside>
</template>

<script setup>
import { t } from '../i18n'
import { ref, watch } from 'vue'
import ConfirmDialog from './ConfirmDialog.vue'
import DateField from './DateField.vue'

const props = defineProps({
  hex: { type: Object, required: true }, // { q, r }
  panel: { type: Object, required: true }, // { majors, minors, kingdom }
  allMajors: { type: Array, default: () => [] },
  unplaced: { type: Array, default: () => [] },
  // Async callbacks from the map; they throw with a message on failure.
  saveMinor: { type: Function, required: true },
  deleteMinor: { type: Function, required: true },
  placeLocation: { type: Function, required: true },
})
defineEmits(['close', 'edit-major'])

const formOpen = ref(false)
const editingId = ref(null)
const name = ref('')
const founding = ref('')
const desc = ref('')
const belongs = ref('') // '' = automatic, otherwise a major location's id
const isCity = ref(false)
const isCapital = ref(false)
const saving = ref(false)
const error = ref('')
const confirmTarget = ref(null)

function openAdd() {
  editingId.value = null
  name.value = ''
  founding.value = ''
  desc.value = ''
  belongs.value = ''
  isCity.value = false
  isCapital.value = false
  error.value = ''
  formOpen.value = true
}

function openEdit(m) {
  editingId.value = m.id
  name.value = m.name
  founding.value = m.founding_date
  desc.value = m.description
  belongs.value = m.belongs_to_id ?? ''
  isCity.value = !!m.is_city
  isCapital.value = !!m.is_capital
  error.value = ''
  formOpen.value = true
}

function closeForm() {
  formOpen.value = false
  editingId.value = null
  error.value = ''
}

// A different hex means a different context; drop any half-filled form.
watch(() => [props.hex.q, props.hex.r], closeForm)

async function run(fn) {
  error.value = ''
  try {
    await fn()
    return true
  } catch (err) {
    error.value = err.message || t('Something went wrong.')
    return false
  }
}

async function submit() {
  if (!name.value.trim()) return
  saving.value = true
  const ok = await run(() =>
    props.saveMinor({
      id: editingId.value,
      name: name.value.trim(),
      founding_date: founding.value,
      description: desc.value,
      belongs_to_id: belongs.value === '' ? null : belongs.value,
      is_city: isCity.value,
      is_capital: isCapital.value,
    }),
  )
  saving.value = false
  if (ok) closeForm()
}

function place(m) {
  run(() => props.placeLocation(m))
}

async function confirmDelete() {
  const target = confirmTarget.value
  confirmTarget.value = null
  await run(() => props.deleteMinor(target))
}

function belongsLabel(m) {
  if (m.belongs_to_id) {
    const parent = props.allMajors.find((x) => x.id === m.belongs_to_id)
    if (parent) return t('{name} (set manually)', { name: parent.name })
  }
  return props.panel.kingdom ? props.panel.kingdom.name : t('No kingdom')
}
</script>

<style scoped>
.hex-panel {
  position: absolute;
  top: 16px;
  left: 92px;
  width: 330px;
  max-height: calc(100% - 100px);
  overflow-y: auto;
  padding: 1rem 1.1rem 1.2rem;
  z-index: 6;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 0.8rem;
}

.panel-head h3 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.05rem;
  margin: 0;
  color: var(--text-primary);
}

.coords {
  margin: 0.1rem 0 0;
  font-size: 0.75rem;
  color: var(--text-faint);
}

.icon-btn {
  background: transparent;
  border: none;
  color: var(--text-faint);
  cursor: pointer;
  font-size: 0.9rem;
  padding: 0.2rem 0.4rem;
}

.icon-btn:hover {
  color: var(--text-primary);
}

section {
  margin-top: 1.1rem;
}

h4 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 0.9rem;
  margin: 0 0 0.5rem;
  color: var(--text-primary);
}

.section-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}

.section-row h4 {
  margin: 0;
}

.muted {
  margin: 0 0 0.5rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--text-faint);
}

.mini-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.mini-list li {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.6rem;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--glass-border);
  border-radius: 8px;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot.none {
  border: 1.5px dashed var(--text-faint);
}

.mini-name {
  flex: 1;
  min-width: 0;
  font-size: 0.85rem;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mini-tag {
  font-size: 0.68rem;
  color: var(--text-faint);
}

.minor-item {
  justify-content: space-between;
}

.minor-main {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
  flex: 1;
}

.badge {
  margin-left: 0.35rem;
  padding: 0.05rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--glass-border);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.badge.is-capital {
  color: var(--accent);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.settlement {
  margin-bottom: 1rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 1.1rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.settlement-title {
  width: 100%;
}

.check {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--text-primary);
}

.settlement-hint {
  width: 100%;
  margin: 0;
  font-size: 0.76rem;
  color: var(--text-faint);
}

.belongs {
  font-size: 0.72rem;
  color: var(--text-faint);
}

.minor-actions {
  display: flex;
  gap: 0.35rem;
  flex-shrink: 0;
}

.minor-form {
  margin-top: 0.8rem;
  padding-top: 0.8rem;
  border-top: 1px solid var(--glass-border);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}

/* Phones: a sheet across the bottom of the map, above the location bar. */
@media (max-width: 720px) {
  .hex-panel {
    top: auto;
    left: 8px;
    right: 8px;
    bottom: 72px;
    width: auto;
    max-height: 55%;
    z-index: 8;
  }
}
</style>
