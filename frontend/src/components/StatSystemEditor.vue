<template>
  <div class="stat-editor">
    <section class="glass-panel card">
      <h2>{{ $t('Stats') }}</h2>
      <p class="lede">
        {{ $t('Which stats your characters have and how each one is calculated. The character sheet, the bonuses of classes, races and gear, and version comparisons all follow this. Saving updates every character at once.') }}
      </p>

      <p v-if="loadError" class="error-banner">{{ loadError }}</p>

      <div v-else class="sample">
        <span class="sample-title">{{ $t('Preview with a sample character') }}</span>
        <div class="sample-fields">
          <label class="field">
            <span>{{ $t('Level') }}</span>
            <input v-model.number="sample.level" type="number" min="1" step="1" />
          </label>
          <label class="field">
            <span>{{ $t('Age') }}</span>
            <input v-model.number="sample.age" type="number" step="1" />
          </label>
          <label class="field">
            <span>{{ $t('Height (cm)') }}</span>
            <input v-model.number="sample.height" type="number" step="1" />
          </label>
          <label class="field">
            <span>{{ $t('Weight (kg)') }}</span>
            <input v-model.number="sample.weight" type="number" step="1" />
          </label>
          <label class="field">
            <span>{{ $t('Points in each primary stat') }}</span>
            <input v-model.number="sample.points" type="number" min="0" step="1" />
          </label>
          <label class="field">
            <span>{{ $t('Each typed value') }}</span>
            <input v-model.number="sample.typed" type="number" step="any" />
          </label>
          <label class="field" :title="$t('As if the class, subclass, specialization, race, body type and gear each gave this much to every calculated stat.')">
            <span>{{ $t('Each bonus') }}</span>
            <input v-model.number="sample.bonus" type="number" step="any" />
          </label>
        </div>
      </div>
    </section>

    <template v-if="loaded">
      <section v-for="cat in STAT_CATEGORIES" :key="cat.id" class="glass-panel card category">
        <h2>{{ $t(cat.label) }}</h2>
        <p class="lede">{{ $t(cat.about) }}</p>

        <div v-if="cat.id === 'primary'" class="points">
          <div class="points-head">
            <span class="points-title">{{ $t('Points to spend') }}</span>
            <button v-if="canRestore(null, 'points_formula')" type="button" class="link-btn restore-link" @click="askRestore(null, 'points_formula')">{{ $t('Restore default') }}</button>
            <span class="preview-value">{{ $tn('{n} point', '{n} points', preview.points ?? 0) }}</span>
          </div>
          <FormulaEditor
            v-model="draft.points_formula"
            :values="pointsValues"
            :toolbar="active === 'points'"
            :invalid="!!issuesFor('', 'points').length"
            :label="$t('Points to spend')"
            @focus="active = 'points'"
          />
          <p v-for="(msg, i) in issuesFor('', 'points')" :key="i" class="issue">{{ msg }}</p>
        </div>

        <ul class="stat-list">
          <li
            v-for="stat in statsIn(cat.id, draft.stats)"
            :key="stat.key"
            class="stat-item"
            :class="{ 'is-open': open === stat.key, 'has-issue': issuesFor(stat.key).length }"
          >
            <div class="stat-row" @click="toggle(stat.key)">
              <div class="order" @click.stop>
                <button type="button" class="icon-btn" :aria-label="$t('Move up')" :disabled="!canMove(stat, -1)" @click="move(stat, -1)">▲</button>
                <button type="button" class="icon-btn" :aria-label="$t('Move down')" :disabled="!canMove(stat, 1)" @click="move(stat, 1)">▼</button>
              </div>
              <span class="stat-name">
                {{ statName(stat) || $t('(no name)') }}
                <span v-if="stat.short_name" class="short">{{ $t(stat.short_name) }}</span>
              </span>
              <span v-if="cat.id !== 'primary'" class="row-formula">
                <FormulaEditor :model-value="stat.formula" :values="valuesFor(stat)" readonly :placeholder="$t('0')" />
              </span>
              <span v-else class="row-formula muted">{{ $t('Points plus bonuses') }}</span>
              <span v-if="cat.id !== 'primary'" class="preview-value" :title="$t('For the sample character, without bonuses')">
                {{ formatStat(stat, preview.values[stat.key]) }}
              </span>
              <span v-if="issuesFor(stat.key).length" class="issue-dot" :title="issuesFor(stat.key)[0]">!</span>
            </div>

            <div v-if="open === stat.key" class="stat-body">
              <div class="field-grid">
                <label class="field">
                  <span>{{ $t('Name') }}</span>
                  <input :value="stat.name" type="text" maxlength="60" @input="rename(stat, $event.target.value)" />
                </label>
                <label class="field">
                  <span>{{ $t('Short name') }}</span>
                  <input v-model="stat.short_name" type="text" maxlength="12" :placeholder="cat.id === 'primary' ? $t('e.g. VIT') : $t('Optional')" />
                </label>
                <template v-if="cat.id !== 'primary'">
                  <label class="field">
                    <span>{{ $t('Unit') }}</span>
                    <input v-model="stat.unit" type="text" maxlength="12" :placeholder="$t('e.g. kg, x, °C')" />
                  </label>
                  <label class="field">
                    <span>{{ $t('Decimals') }}</span>
                    <select v-model.number="stat.decimals">
                      <option v-for="d in [0, 1, 2, 3, 4]" :key="d" :value="d">{{ d }}</option>
                    </select>
                  </label>
                  <label class="field">
                    <span>{{ $t('Lowest value') }}</span>
                    <input :value="stat.min ?? ''" type="number" step="any" :placeholder="$t('None')" @input="setBound(stat, 'min', $event.target.value)" />
                  </label>
                  <label class="field check-field" :title="$t('A drop is good news, as for a cold threshold: negative bonuses and falling values are shown as improvements.')">
                    <input v-model="stat.lower_is_better" type="checkbox" />
                    <span>{{ $t('Lower is better') }}</span>
                  </label>
                  <label class="field">
                    <span>{{ $t('Highest value') }}</span>
                    <input :value="stat.max ?? ''" type="number" step="any" :placeholder="$t('None')" @input="setBound(stat, 'max', $event.target.value)" />
                  </label>
                </template>
              </div>
              <p v-for="(msg, i) in issuesFor(stat.key, ['name', 'key', 'min'])" :key="'f' + i" class="issue">{{ msg }}</p>

              <template v-if="cat.id !== 'primary'">
                <div class="formula-field">
                  <div class="formula-head">
                    <span class="formula-title">{{ $t('Formula') }}</span>
                    <button v-if="canRestore(stat, 'formula')" type="button" class="link-btn restore-link" @click="askRestore(stat, 'formula')">{{ $t('Restore default') }}</button>
                  </div>
                  <FormulaEditor
                    v-model="stat.formula"
                    :values="valuesFor(stat)"
                    :toolbar="active === 'formula'"
                    :invalid="!!issuesFor(stat.key, 'formula').length"
                    :label="$t('Formula')"
                    @focus="active = 'formula'"
                  />
                  <p v-for="(msg, i) in issuesFor(stat.key, 'formula')" :key="i" class="issue">{{ msg }}</p>
                </div>

                <div v-if="stat.multiplier || multiplierOpen === stat.key" class="formula-field">
                  <div class="formula-head">
                    <span class="formula-title">{{ $t('Multiplier') }}</span>
                    <button v-if="canRestore(stat, 'multiplier')" type="button" class="link-btn restore-link" @click="askRestore(stat, 'multiplier')">{{ $t('Restore default') }}</button>
                  </div>
                  <p class="note">{{ $t('Optional. The stat, bonuses included, is multiplied by 1 + this: 0.25 makes it 25% higher.') }}</p>
                  <FormulaEditor
                    v-model="stat.multiplier"
                    :values="valuesFor(stat)"
                    :toolbar="active === 'multiplier'"
                    :invalid="!!issuesFor(stat.key, 'multiplier').length"
                    :label="$t('Multiplier')"
                    @focus="active = 'multiplier'"
                  />
                  <p v-for="(msg, i) in issuesFor(stat.key, 'multiplier')" :key="i" class="issue">{{ msg }}</p>
                </div>
                <button v-else type="button" class="link-btn" @click="multiplierOpen = stat.key; active = 'multiplier'">
                  {{ $t('Add a multiplier') }}
                </button>

                <p class="note">
                  {{ readsBase(stat)
                    ? $t('This formula uses Typed value, so the character sheet has a box for typing it.')
                    : $t('Add Typed value to the formula to get a box on the character sheet for typing this stat.') }}
                </p>
              </template>

              <div class="stat-actions">
                <span v-if="usageText(stat)" class="usage">{{ usageText(stat) }}</span>
                <button type="button" class="btn btn-ghost small danger-text" @click="remove(stat)">{{ $t('Remove stat') }}</button>
              </div>
            </div>
          </li>
        </ul>

        <button type="button" class="btn btn-ghost small" @click="add(cat.id)">+ {{ $t('Add stat') }}</button>
      </section>

      <div class="save-bar glass-panel panel-solid">
        <span v-if="issueCount" class="issue-count">{{ $tn('{n} problem to fix', '{n} problems to fix', issueCount) }}</span>
        <span v-else-if="saved" class="saved-hint">{{ $t('Saved.') }}</span>
        <span v-else-if="dirty()" class="unsaved">{{ $t('Unsaved changes') }}</span>
        <p v-if="saveError" class="save-error">{{ saveError }}</p>
        <div class="save-actions">
          <button type="button" class="btn btn-ghost small" :disabled="saving" @click="confirming = 'defaults'">{{ $t('Back to the defaults') }}</button>
          <button type="button" class="btn btn-ghost small" :disabled="saving || !dirty()" @click="discard">{{ $t('Discard changes') }}</button>
          <button type="button" class="btn btn-primary small" :disabled="saving || !dirty()" @click="askSave">
            {{ saving ? $t('Saving…') : $t('Save stats') }}
          </button>
        </div>
      </div>
    </template>
    <p v-else-if="!loadError" class="loading-hint">{{ $t('Loading…') }}</p>

    <RestoreDefaultDialog
      v-if="confirming === 'defaults'"
      :title="$t('Back to the defaults?')"
      :message="$t('Every stat and formula is replaced with the ones Polaris starts with. Nothing changes until you save, and saving removes what was typed for, and the bonuses given to, stats that are no longer there.')"
      @cancel="confirming = ''"
      @confirm="useDefaults"
    />
    <RestoreDefaultDialog
      v-if="restoreTarget"
      :title="$t('Restore the default formula?')"
      :message="restoreMessage"
      @cancel="restoreTarget = null"
      @confirm="restoreFormula"
    />
    <ConfirmDialog
      v-if="confirming === 'remove'"
      :title="$t('Remove these stats?')"
      :message="removalMessage"
      :confirm-label="$t('Save and remove')"
      @cancel="confirming = ''"
      @confirm="save"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { t, tn } from '../i18n'
import { statSystemApi } from '../api'
import {
  STAT_CATEGORIES, FORMULA_INPUTS, FORMULA_BONUSES, BONUS_ABOUT, applyStatSystem, statsIn, statName, statShort, readsBase, formatStat,
  keyFromName, renameRef, issueText,
} from '../stats'
import { useUnsavedGuard, useSaveShortcut } from '../navigation'
import FormulaEditor from './FormulaEditor.vue'
import ConfirmDialog from './ConfirmDialog.vue'
import RestoreDefaultDialog from './RestoreDefaultDialog.vue'

// Settings → Stats: edits the story's stat system (backend/statsystem.go) as
// a draft, previews it on a sample character while you work, and saves it
// whole.

const loaded = ref(false)
const loadError = ref('')
const saving = ref(false)
const saved = ref(false)
const saveError = ref('')
const confirming = ref('')

const draft = reactive({ points_formula: '', stats: [] })
let savedState = '' // the saved system, as JSON
const savedKeys = ref(new Set())
const usage = ref({})

const open = ref('') // the stat being edited
const active = ref('formula') // which formula of it has the toolbar
const multiplierOpen = ref('')

const sample = reactive({ level: 10, age: 18, height: 170, weight: 70, points: 5, typed: 0, bonus: 0 })
const preview = ref({ issues: [], points: 0, values: {} })
const serverIssues = ref([]) // from the last save, until the draft changes

const state = () => JSON.stringify({ points_formula: draft.points_formula, stats: draft.stats })
const dirty = () => loaded.value && state() !== savedState
useUnsavedGuard(dirty)
useSaveShortcut(() => loaded.value && dirty() && !saving.value && askSave())

function setDraft(system) {
  draft.points_formula = system.points_formula || ''
  draft.stats = (system.stats || []).map((s) => ({ ...s, lower_is_better: !!s.lower_is_better, short_name: s.short_name || '', formula: s.formula || '', multiplier: s.multiplier || '', unit: s.unit || '' }))
}

function takeSaved(system) {
  setDraft(system)
  savedState = state()
  savedKeys.value = new Set(draft.stats.map((s) => s.key))
  usage.value = system.usage || {}
}

onMounted(async () => {
  try {
    takeSaved(await statSystemApi.get())
    loaded.value = true
  } catch (err) {
    loadError.value = t("Couldn't load the stats. Try refreshing.")
  }
  try {
    defaults.value = await statSystemApi.defaults()
  } catch (err) {
    // Without them, there is nothing to restore a formula to.
  }
})

// ---- Restoring one formula --------------------------------------------------
// The system Polaris starts with: a stat it also has (same key) can have its
// formula or multiplier put back on its own, into the draft.
const defaults = ref(null)
const restoreTarget = ref(null) // { stat, field }, stat null for the points formula

function defaultFor(stat, field) {
  if (!defaults.value) return null
  if (!stat) return defaults.value.points_formula || ''
  const d = (defaults.value.stats || []).find((s) => s.key === stat.key && s.category === stat.category)
  return d ? d[field] || '' : null
}

function canRestore(stat, field) {
  const d = defaultFor(stat, field)
  if (d === null) return false
  const now = stat ? stat[field] : draft[field]
  return (now || '').trim() !== d.trim()
}

function askRestore(stat, field) {
  restoreTarget.value = { stat, field }
}

const restoreMessage = computed(() => {
  const r = restoreTarget.value
  if (!r) return ''
  const formula = defaultFor(r.stat, r.field)
  const name = r.stat ? statName(r.stat) : t('Points to spend')
  return formula
    ? t('{name} goes back to the default: {formula}. Nothing changes until you save the stats.', { name, formula })
    : t('{name} goes back to the default, which is empty. Nothing changes until you save the stats.', { name })
})

function restoreFormula() {
  const { stat, field } = restoreTarget.value
  restoreTarget.value = null
  const d = defaultFor(stat, field)
  if (d === null) return
  if (stat) stat[field] = d
  else draft[field] = d
}

// ---- What formulas can read -------------------------------------------------

const inputItem = (i) => ({ key: i.key, label: t(i.label), title: t(i.about), input: true })

const pointsValues = computed(() => [
  { label: t('Character'), items: FORMULA_INPUTS.filter((i) => i.key !== 'base').map(inputItem) },
])

// A calculated stat can read the character, its own typed value and
// bonuses, every primary stat and every other calculated stat.
function valuesFor(stat) {
  return [
    { label: t('Character'), items: FORMULA_INPUTS.map(inputItem) },
    { label: t('Bonuses'), items: FORMULA_BONUSES.map((b) => ({ key: b.key, label: t(b.label), title: t(BONUS_ABOUT), input: true })) },
    ...STAT_CATEGORIES.map((c) => ({
      label: t(c.label),
      items: statsIn(c.id, draft.stats)
        .filter((s) => s.key !== stat.key)
        .map((s) => ({ key: s.key, label: statShort(s) || s.key, title: statName(s) })),
    })),
  ]
}

// ---- Problems -----------------------------------------------------------------

const allIssues = computed(() => (serverIssues.value.length ? serverIssues.value : preview.value.issues || []))
const issueCount = computed(() => allIssues.value.length)
const nameOfKey = (k) => {
  const s = draft.stats.find((x) => x.key === k)
  return s ? statShort(s) : k
}

function issuesFor(key, fields) {
  const want = fields ? [].concat(fields) : null
  return allIssues.value
    .filter((i) => i.key === key && (!want || want.includes(i.field)))
    .map((i) => issueText(i, nameOfKey))
}

// ---- Live preview -------------------------------------------------------------

let timer = null
let seq = 0
function schedulePreview() {
  clearTimeout(timer)
  timer = setTimeout(runPreview, 300)
}

async function runPreview() {
  const n = ++seq
  const values = {}
  for (const s of draft.stats) values[s.key] = s.category === 'primary' ? sample.points || 0 : sample.typed || 0
  try {
    const result = await statSystemApi.preview(
      { points_formula: draft.points_formula, stats: draft.stats },
      { level: sample.level || 1, age: sample.age || 0, height: sample.height || 0, weight: sample.weight || 0, values, bonus: sample.bonus || 0 },
    )
    if (n === seq) preview.value = { issues: result.issues || [], points: result.points, values: result.values || {} }
  } catch (err) {
    // Keep the last preview.
  }
}

watch([draft, sample], () => {
  serverIssues.value = []
  if (dirty()) saved.value = false
  schedulePreview()
}, { deep: true })

// ---- Editing --------------------------------------------------------------------

function toggle(key) {
  open.value = open.value === key ? '' : key
  active.value = 'formula'
}

// A stat that isn't saved yet takes its key from its name, and every formula
// that already reads it follows. A saved stat's key never changes.
function rename(stat, name) {
  stat.name = name
  if (savedKeys.value.has(stat.key)) return
  const taken = draft.stats.filter((s) => s !== stat).map((s) => s.key)
  const key = keyFromName(name, taken)
  if (key === stat.key) return
  const from = stat.key
  for (const s of draft.stats) {
    s.formula = renameRef(s.formula, from, key)
    s.multiplier = renameRef(s.multiplier, from, key)
  }
  stat.key = key
  if (open.value === from) open.value = key
}

function setBound(stat, which, raw) {
  const n = parseFloat(raw)
  stat[which] = raw === '' || Number.isNaN(n) ? null : n
}

const NEW_STAT = {
  primary: { formula: '', unit: '', decimals: 0 },
  base: { formula: '', unit: '', decimals: 0 },
  special: { formula: '', unit: '', decimals: 0 },
  defense: { formula: '1 + [base]', unit: 'x', decimals: 2 },
  lifeskill: { formula: '[base]', unit: '', decimals: 0 },
}

function add(category) {
  const name = t('New stat')
  const stat = {
    key: keyFromName(name, draft.stats.map((s) => s.key)),
    category,
    name,
    short_name: '',
    multiplier: '',
    min: null,
    max: null,
    lower_is_better: false,
    ...NEW_STAT[category],
  }
  // After the last stat of its category.
  const last = draft.stats.map((s) => s.category).lastIndexOf(category)
  draft.stats.splice(last < 0 ? draft.stats.length : last + 1, 0, stat)
  open.value = stat.key
  active.value = 'formula'
}

function remove(stat) {
  draft.stats.splice(draft.stats.indexOf(stat), 1)
  if (open.value === stat.key) open.value = ''
}

function neighbour(stat, dir) {
  const same = draft.stats.filter((s) => s.category === stat.category)
  return same[same.indexOf(stat) + dir] || null
}
const canMove = (stat, dir) => !!neighbour(stat, dir)

function move(stat, dir) {
  const other = neighbour(stat, dir)
  if (!other) return
  const i = draft.stats.indexOf(stat)
  const j = draft.stats.indexOf(other)
  draft.stats.splice(i, 1, other)
  draft.stats.splice(j, 1, stat)
}

function usageText(stat) {
  const u = usage.value[stat.key]
  if (!u || (!u.versions && !u.bonuses)) return ''
  return [
    u.versions ? tn('Typed on {n} character version', 'Typed on {n} character versions', u.versions) : '',
    u.bonuses ? tn('{n} bonus', '{n} bonuses', u.bonuses) : '',
  ].filter(Boolean).join(' · ')
}

// ---- Saving --------------------------------------------------------------------

const removedInUse = computed(() => {
  const now = new Set(draft.stats.map((s) => s.key))
  const before = JSON.parse(savedState || '{"stats":[]}').stats
  return before.filter((s) => !now.has(s.key) && usageText(s))
})

const removalMessage = computed(() =>
  t('Saving removes what was typed for these stats and the bonuses given to them: {list}.', {
    list: removedInUse.value.map((s) => `${statName(s)} (${usageText(s)})`).join('; '),
  }),
)

function askSave() {
  if (removedInUse.value.length) confirming.value = 'remove'
  else save()
}

async function save() {
  confirming.value = ''
  saving.value = true
  saveError.value = ''
  try {
    const result = await statSystemApi.save({ points_formula: draft.points_formula, stats: draft.stats })
    takeSaved(result)
    applyStatSystem(result)
    saved.value = true
    setTimeout(() => (saved.value = false), 2500)
  } catch (err) {
    if (err.issues) {
      serverIssues.value = err.issues
      const first = err.issues.find((i) => i.key)
      if (first) open.value = first.key
    } else saveError.value = err.message || t("Couldn't save. Try again.")
  } finally {
    saving.value = false
  }
}

function discard() {
  setDraft(JSON.parse(savedState))
  open.value = ''
}

async function useDefaults() {
  confirming.value = ''
  try {
    setDraft(await statSystemApi.defaults())
    open.value = ''
  } catch (err) {
    saveError.value = err.message || t("Couldn't load the stats. Try refreshing.")
  }
}
</script>

<style scoped>
.card {
  padding: 1.75rem;
  margin-bottom: 1.25rem;
}

.card h2 {
  margin-top: 0;
}

.lede {
  color: var(--text-muted);
  font-size: 0.88rem;
  line-height: 1.5;
}

.sample {
  margin-top: 1rem;
}

.formula-head {
  display: flex;
  align-items: baseline;
  gap: 0.75rem;
}

.link-btn.restore-link {
  font-size: 0.74rem;
}

.formula-head .restore-link,
.points-head .restore-link {
  margin-left: auto;
}

.points-head .restore-link {
  margin-right: 0.75rem;
}

.sample-title,
.points-title,
.formula-title {
  display: block;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
  margin-bottom: 0.5rem;
}

.sample-fields {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 0 0.75rem;
}

.sample-fields .field {
  margin-bottom: 0.5rem;
}

.points {
  margin-bottom: 1rem;
}

.points-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

.stat-list {
  list-style: none;
  margin: 0 0 1rem;
  padding: 0;
}

.stat-item {
  border-top: 1px solid var(--glass-border);
}

.stat-item:last-child {
  border-bottom: 1px solid var(--glass-border);
}

.stat-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.55rem 0.25rem;
  cursor: pointer;
}

.stat-row:hover {
  background: rgba(255, 255, 255, 0.025);
}

.order {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.icon-btn {
  background: transparent;
  border: none;
  color: var(--text-faint);
  font-size: 0.55rem;
  line-height: 1;
  padding: 0.1rem 0.25rem;
  cursor: pointer;
}

.icon-btn:not(:disabled):hover {
  color: var(--accent);
}

.icon-btn:disabled {
  opacity: 0.25;
  cursor: default;
}

.stat-name {
  flex: 0 0 11rem;
  min-width: 0;
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.88rem;
}

.short {
  margin-left: 0.35rem;
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--text-faint);
}

.row-formula {
  flex: 1;
  min-width: 0;
}

.muted {
  font-size: 0.82rem;
  color: var(--text-faint);
}

.preview-value {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--accent);
  font-size: 0.85rem;
  white-space: nowrap;
}

.issue-dot {
  display: inline-grid;
  place-items: center;
  width: 1.2rem;
  height: 1.2rem;
  border-radius: 50%;
  background: color-mix(in srgb, var(--danger) 25%, transparent);
  color: var(--danger-text);
  font-size: 0.72rem;
  font-weight: 800;
}

.stat-item.is-open {
  background: rgba(255, 255, 255, 0.02);
}

.stat-body {
  padding: 0.5rem 0.5rem 1rem 2rem;
}

.check-field {
  flex-direction: row;
  align-items: center;
  align-self: end;
  gap: 0.5rem;
  min-height: var(--control-h);
}

.formula-field {
  margin: 0.25rem 0 0.75rem;
}

.note {
  font-size: 0.78rem;
  color: var(--text-faint);
  margin: 0.4rem 0;
}

.issue {
  font-size: 0.8rem;
  color: var(--danger-text);
  margin: 0.35rem 0 0;
}

.link-btn {
  background: none;
  border: none;
  padding: 0;
  color: var(--accent);
  font-size: 0.82rem;
  cursor: pointer;
}

.stat-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.usage {
  font-size: 0.78rem;
  color: var(--text-faint);
}

.danger-text {
  color: var(--danger-text);
}

.save-bar {
  position: sticky;
  bottom: 1rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  z-index: 5;
}

.save-actions {
  display: flex;
  gap: 0.5rem;
  margin-left: auto;
  flex-wrap: wrap;
}

.issue-count,
.save-error {
  color: var(--danger-text);
  font-size: 0.85rem;
  margin: 0;
}

.unsaved {
  color: var(--text-muted);
  font-size: 0.85rem;
}

.saved-hint {
  color: var(--accent);
  font-size: 0.85rem;
}

@media (max-width: 720px) {
  .card {
    padding: 1.25rem;
  }
  .stat-row {
    flex-wrap: wrap;
  }
  .stat-name {
    flex: 1 1 auto;
  }
  .row-formula {
    flex-basis: 100%;
    order: 5;
  }
  .stat-body {
    padding-left: 0.5rem;
  }
}
</style>
