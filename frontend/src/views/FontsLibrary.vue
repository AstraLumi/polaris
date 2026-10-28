<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Fonts') }}</h1>
        <p class="page-sub">{{ $t('Your own font files for the writing editor, shared by every story, each with a record of its license.') }}</p>
      </div>
    </header>

    <section class="glass-panel upload" :aria-label="$t('Add fonts')">
      <FilePick ref="pick" multiple accept=".ttf,.otf,.woff,.woff2,font/ttf,font/otf,font/woff,font/woff2" :label="$t('Add font files')" @change="onPick($event)" />
      <p class="hint">
        {{ $t('TrueType, OpenType, WOFF or WOFF2, up to 40 MB each. Add a family\'s styles together (regular, bold, italic…): they are grouped by the name inside the file.') }}
      </p>
      <p v-if="uploading" class="hint">{{ $t('Adding…') }}</p>
      <ul v-if="problems.length" class="problems">
        <li v-for="p in problems" :key="p.file">{{ p.file }}: {{ $t(p.error) }}</li>
      </ul>
      <label class="field sample">
        <span>{{ $t('Sample text') }}</span>
        <input v-model="sample" type="text" maxlength="200" />
      </label>
    </section>

    <p v-if="error" class="error-banner">{{ error }}</p>
    <p v-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="!fontLibrary.length" class="glass-panel empty-state">
      <p class="empty-title">{{ $t('No fonts yet') }}</p>
      <p class="empty-hint">{{ $t('Add the font files you bought or downloaded. The writing editor offers them first, and the e-book export carries the ones their license allows.') }}</p>
    </div>

    <ul v-else class="families">
      <li v-for="fam in fontLibrary" :key="fam.id" class="glass-panel family">
        <div class="fam-head">
          <h2>{{ fam.name }}</h2>
          <span class="kind" :class="{ 'is-unset': !fam.license_kind }">{{ $t(LICENSE_LABELS[fam.license_kind] || LICENSE_LABELS.other) }}</span>
          <span class="spacer"></span>
          <button type="button" class="btn btn-ghost small" @click="editing = fam">{{ $t('License…') }}</button>
          <RowMenu
            :items="[
              { label: $t('Add styles…'), action: () => addTo(fam) },
              fam.license_file && { label: $t('Open the receipt or license'), action: () => openLicense(fam) },
              { label: $t('Delete'), danger: true, action: () => (deleting = fam) },
            ]"
          />
        </div>

        <p class="preview" :style="{ fontFamily: cssFamily(fam.name) }">{{ sample || defaultSample }}</p>

        <ul class="faces">
          <li v-for="f in fam.files" :key="f.id" class="face" :title="f.original_name">
            <span :style="{ fontFamily: cssFamily(fam.name), fontWeight: f.weight, fontStyle: f.italic ? 'italic' : 'normal' }">Aa</span>
            {{ faceName(f) }} · {{ f.format.toUpperCase() }}
            <span v-if="!f.details_read" class="unread" :title="$t('Couldn\'t read details from this file (WOFF2); check its weight and style.')">?</span>
          </li>
          <li v-if="!fam.files.length" class="face is-empty">{{ $t('No files') }}</li>
        </ul>

        <ul class="uses">
          <li v-for="u in USES" :key="u" :class="{ 'is-yes': fam.uses[u] }">{{ fam.uses[u] ? '✓' : '✕' }} {{ $t(USE_LABELS[u]) }}</li>
        </ul>

        <ul v-if="warningsOf(fam).length" class="warns">
          <li v-for="w in warningsOf(fam)" :key="w">⚠ {{ warningLine(fam, w) }}</li>
        </ul>

        <div class="used">
          <span class="label">{{ $t('Used in') }}</span>
          <span v-if="used === null" class="faint">{{ $t('Looking…') }}</span>
          <span v-else-if="!usesOf(fam).length" class="faint">{{ $t('Nothing yet') }}</span>
          <ul v-else>
            <li v-for="(u, i) in usesOf(fam)" :key="i">
              <strong>{{ u.story }}</strong> › {{ u.book || $t('Chapters without a volume') }}
              <span class="where">{{ whereText(u) }}</span>
            </li>
          </ul>
        </div>
      </li>
    </ul>

    <FilePick v-show="false" ref="addPick" multiple accept=".ttf,.otf,.woff,.woff2" @change="onPick($event, addFamily)" />

    <FontLicenseModal v-if="editing" :family="editing" @close="editing = null" @saved="editing = null" />

    <Teleport to="body">
      <ConfirmDialog
        v-if="deleting"
        :title="$t('Delete {name}?', { name: deleting.name })"
        :message="deleteMessage"
        :confirm-label="$t('Delete')"
        @cancel="deleting = null"
        @confirm="remove"
      />
    </Teleport>
  </div>
</template>

<script setup>
// The fonts library (docs/EDITOR.md, "Fonts library"): every family with a
// line in its own font, its files, its license, what to watch out for and
// which stories' books use it. Global: the same list in every story.
import { ref, computed, onMounted } from 'vue'
import { t, tn } from '../i18n'
import FilePick from '../components/FilePick.vue'
import RowMenu from '../components/RowMenu.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import FontLicenseModal from '../components/FontLicenseModal.vue'
import { USES, cssFamily, familyWarnings } from '../fontRules'
import {
  fontLibrary, loadFontLibrary, uploadFonts, deleteFontFamily, fetchFontsUsed, licenseFileUrl, fontWarningText,
  LICENSE_LABELS, USE_LABELS, WEIGHT_NAMES,
} from '../fontLibrary'

const loading = ref(true)
const error = ref('')
const uploading = ref(false)
const problems = ref([])
const used = ref(null) // family id → uses, once scanned
const editing = ref(null)
const deleting = ref(null)
const addFamily = ref(null)
const pick = ref(null)
const addPick = ref(null)
const defaultSample = computed(() => t('The night was quiet, and the old map glowed under the lamp.'))
const sample = ref('')

const usesOf = (fam) => used.value?.[fam.id] || []
const warningsOf = (fam) => familyWarnings(fam, usesOf(fam))
const warningLine = (fam, reason) => fontWarningText({ family: fam.name, reason })

function faceName(f) {
  const w = t(WEIGHT_NAMES[f.weight] || WEIGHT_NAMES[400])
  return f.italic ? t('{weight} italic', { weight: w }) : w
}

function whereText(u) {
  const parts = []
  if (u.styles?.length) parts.push(t('styles: {list}', { list: u.styles.join(', ') }))
  if (u.bands) parts.push(t('headers and footers'))
  if (u.chapters) parts.push(tn('{n} chapter', '{n} chapters', u.chapters, { n: u.chapters }))
  return parts.length ? `(${parts.join('; ')})` : ''
}

async function scanUses() {
  try {
    used.value = await fetchFontsUsed()
  } catch (e) {
    used.value = {}
  }
}

// Files picked or dropped; `family` (from "Add styles…") puts them in it.
async function onPick(e, family = null) {
  const files = [...(e.target.files || [])]
  e.target.value = ''
  addFamily.value = null
  if (!files.length) return
  uploading.value = true
  error.value = ''
  problems.value = []
  try {
    const out = await uploadFonts(files, family?.id)
    problems.value = out.problems || []
    scanUses()
  } catch (err) {
    error.value = err.message
  } finally {
    uploading.value = false
    if (pick.value) pick.value.value = ''
    if (addPick.value) addPick.value.value = ''
  }
}

function addTo(fam) {
  addFamily.value = fam
  addPick.value?.click()
}

const openLicense = (fam) => window.open(licenseFileUrl(fam.id), '_blank', 'noopener')

const deleteMessage = computed(() => {
  const n = deleting.value ? usesOf(deleting.value).length : 0
  if (n) return tn('A book still uses it: its text will show in another font.', '{n} books still use it: their text will show in another font.', n, { n })
  return t('Its files and its receipt are deleted from the library.')
})

async function remove() {
  const fam = deleting.value
  deleting.value = null
  try {
    await deleteFontFamily(fam.id)
  } catch (e) {
    error.value = e.message
  }
}

onMounted(async () => {
  await loadFontLibrary(true)
  loading.value = false
  scanUses()
})
</script>

<style scoped>
.upload {
  padding: 1rem 1.2rem;
  margin-bottom: 1.25rem;
}

.hint {
  margin: 0.5rem 0 0;
  font-size: 0.8rem;
  color: var(--text-faint);
  line-height: 1.45;
}

.problems {
  margin: 0.5rem 0 0;
  padding-left: 1.2rem;
  font-size: 0.82rem;
  color: var(--danger-text);
}

.sample {
  max-width: 34rem;
  margin: 0.9rem 0 0;
}

.families {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 26rem), 1fr));
  gap: 1rem;
}

.family {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 1rem 1.15rem;
  min-width: 0;
}

.fam-head {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.fam-head h2 {
  margin: 0;
  font-size: 1.1rem;
  overflow-wrap: anywhere;
}

.spacer {
  flex: 1;
}

.kind {
  padding: 0.1rem 0.55rem;
  border-radius: 999px;
  font-size: 0.72rem;
  background: var(--accent-soft);
  color: var(--accent);
}

.kind.is-unset {
  background: transparent;
  border: 1px dashed var(--glass-border);
  color: var(--text-faint);
}

.preview {
  margin: 0;
  font-size: 1.45rem;
  line-height: 1.3;
  color: var(--text-primary);
  overflow-wrap: anywhere;
}

.faces,
.uses {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.face {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.15rem 0.55rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  font-size: 0.75rem;
  color: var(--text-muted);
}

.face.is-empty {
  border-style: dashed;
}

.face span:first-child {
  font-size: 0.95rem;
  color: var(--text-primary);
}

.unread {
  color: var(--danger-text);
  font-weight: 700;
}

.uses li {
  font-size: 0.75rem;
  color: var(--text-faint);
}

.uses li.is-yes {
  color: var(--text-muted);
}

.warns {
  margin: 0;
  padding: 0.45rem 0.6rem;
  list-style: none;
  border: 1px solid var(--danger);
  border-radius: 8px;
  font-size: 0.78rem;
  color: var(--danger-text);
  line-height: 1.4;
}

.used {
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.45;
}

.used .label {
  display: block;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-faint);
  margin-bottom: 0.15rem;
}

.used ul {
  margin: 0;
  padding-left: 1rem;
}

.where,
.faint {
  color: var(--text-faint);
}
</style>
