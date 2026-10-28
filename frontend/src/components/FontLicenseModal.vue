<template>
  <div class="overlay" @click.self="requestClose" @pointerdown.capture="touched" @keydown.capture="touched" @input.capture="touched">
    <div class="modal is-wide glass-panel panel-solid">
      <h2>{{ $t('Font and license') }}</h2>

      <form @submit.prevent="submit">
        <label class="field">
          <span>{{ $t('Name') }}</span>
          <input v-model="name" type="text" maxlength="200" required autofocus />
        </label>
        <p class="note">{{ $t('Books find the font by this name, so a style that names it uses these files.') }}</p>

        <div class="field-row">
          <label class="field">
            <span>{{ $t('License') }}</span>
            <select v-model="kind" @change="onKind">
              <option v-for="k in KINDS" :key="k" :value="k">{{ $t(LICENSE_LABELS[k]) }}</option>
            </select>
          </label>
          <label class="field">
            <span>{{ $t('Purchase date') }}</span>
            <input v-model="purchaseDate" type="date" />
          </label>
        </div>

        <label class="field">
          <span>{{ $t('Bought or downloaded from') }}</span>
          <input v-model="source" type="text" maxlength="2000" :placeholder="$t('Where it came from: a shop, a web address, a designer…')" />
        </label>

        <fieldset class="uses">
          <legend>{{ $t('The license allows') }}</legend>
          <label v-for="u in USES" :key="u" class="check">
            <input v-model="uses[u]" type="checkbox" />
            {{ $t(USE_LABELS[u]) }}
          </label>
        </fieldset>

        <div class="field">
          <span>{{ $t('Receipt or license file') }}</span>
          <div v-if="family.license_file && !dropLicense" class="current-file">
            <a :href="licenseFileUrl(family.id)" target="_blank" rel="noopener">{{ family.license_file }}</a>
            <button type="button" class="btn btn-ghost small" @click="dropLicense = true">{{ $t('Remove') }}</button>
          </div>
          <FilePick ref="licensePick" accept=".pdf,.txt,image/*,application/pdf,text/plain" :label="$t('Choose a file')" @change="onLicense" />
          <p class="note">{{ $t('A PDF, a picture or a text file, up to 20 MB. Kept with the font.') }}</p>
        </div>

        <label class="field">
          <span>{{ $t('Notes') }}</span>
          <textarea v-model="notes" rows="3" maxlength="10000"></textarea>
        </label>

        <div class="field">
          <span>{{ $t('Files') }}</span>
          <ul class="files">
            <li v-for="f in files" :key="f.id" class="file">
              <span class="file-name" :title="f.original_name">{{ f.original_name || f.format }}</span>
              <select v-model.number="f.weight" :aria-label="$t('Weight')">
                <option v-for="w in WEIGHTS" :key="w" :value="w">{{ w }} · {{ $t(WEIGHT_NAMES[w]) }}</option>
              </select>
              <label class="check">
                <input v-model="f.italic" type="checkbox" />
                {{ $t('Italic') }}
              </label>
              <p v-if="!f.details_read" class="note warn">{{ $t("Couldn't read details from this file (WOFF2); check its weight and style.") }}</p>
              <details v-else class="details">
                <summary>{{ $t("What the file says") }}</summary>
                <dl>
                  <template v-for="d in detailRows(f)" :key="d.label">
                    <dt>{{ d.label }}</dt>
                    <dd>{{ d.value }}</dd>
                  </template>
                </dl>
              </details>
            </li>
          </ul>
        </div>

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? $t('Saving…') : $t('Save') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
// A library font's record (docs/EDITOR.md, "Fonts library"): its name, the
// license kind, where it came from and when it was bought, what the license
// allows, a receipt or license file kept with it, notes, and each file's
// weight and slant.
import { ref, reactive } from 'vue'
import { t } from '../i18n'
import { useModalGuard } from '../navigation'
import FilePick from './FilePick.vue'
import { LICENSE_KINDS, USES } from '../fontRules'
import { LICENSE_LABELS, USE_LABELS, EMBEDDING_LABELS, WEIGHT_NAMES, saveFontFamily, uploadLicenseFile, removeLicenseFile, licenseFileUrl } from '../fontLibrary'

const props = defineProps({
  family: { type: Object, required: true },
})
const emit = defineEmits(['close', 'saved'])

const KINDS = ['', ...LICENSE_KINDS]
const WEIGHTS = [100, 200, 300, 400, 500, 600, 700, 800, 900]

const f0 = props.family
const name = ref(f0.name)
const kind = ref(f0.license_kind || '')
const source = ref(f0.source || '')
const purchaseDate = ref(f0.purchase_date || '')
const notes = ref(f0.notes || '')
const uses = reactive({ ...{ print: false, ebook: false, web: false, commercial: false }, ...(f0.uses || {}) })
const files = ref((f0.files || []).map((f) => ({ ...f })))
const licenseFile = ref(null)
const licensePick = ref(null)
const dropLicense = ref(false)
const saving = ref(false)
const error = ref('')

const { requestClose, touched } = useModalGuard(
  () => [name.value, kind.value, source.value, purchaseDate.value, notes.value, { ...uses }, files.value, licenseFile.value, dropLicense.value],
  () => emit('close'),
)

// The open licenses allow every use; personal use allows none of them.
function onKind() {
  if (kind.value === 'ofl' || kind.value === 'apache') USES.forEach((u) => (uses[u] = true))
  if (kind.value === 'personal') USES.forEach((u) => (uses[u] = false))
}

function onLicense(e) {
  licenseFile.value = e.target.files[0] || null
  if (licenseFile.value) dropLicense.value = false
}

function detailRows(f) {
  return [
    { label: t('Style'), value: f.style },
    { label: t('Full name'), value: f.full_name },
    { label: t('Format'), value: `${f.format.toUpperCase()} · ${Math.max(1, Math.round(f.size / 1024))} KB` },
    { label: t('Embedding'), value: t(EMBEDDING_LABELS[f.embedding] || EMBEDDING_LABELS.unknown) },
    { label: t('Copyright'), value: f.copyright },
    { label: t('License'), value: f.license },
    { label: t('License address'), value: f.license_url },
    { label: t('Designer'), value: f.designer },
    { label: t('Maker'), value: f.vendor },
  ].filter((d) => d.value)
}

async function submit() {
  saving.value = true
  error.value = ''
  try {
    let saved = await saveFontFamily(f0.id, {
      name: name.value,
      license_kind: kind.value,
      source: source.value,
      purchase_date: purchaseDate.value,
      notes: notes.value,
      uses: { ...uses },
      files: files.value.map((f) => ({ id: f.id, weight: f.weight, italic: !!f.italic })),
    })
    if (licenseFile.value) saved = await uploadLicenseFile(f0.id, licenseFile.value)
    else if (dropLicense.value && f0.license_file) saved = await removeLicenseFile(f0.id)
    emit('saved', saved)
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.note {
  margin: -0.35rem 0 0.85rem;
  font-size: 0.78rem;
  color: var(--text-faint);
  line-height: 1.4;
}

.note.warn {
  margin: 0.3rem 0 0;
  color: var(--danger-text);
}

.uses {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem 1.2rem;
  margin: 0 0 1rem;
  padding: 0.6rem 0.8rem;
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-control);
}

.uses legend {
  padding: 0 0.3rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.check {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.85rem;
}

.current-file {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 0.4rem;
  font-size: 0.85rem;
}

.current-file a {
  color: var(--accent);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.files {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.file {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.8rem;
  padding: 0.5rem 0.6rem;
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-control);
}

.file-name {
  flex: 1 1 10rem;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.85rem;
}

.file select {
  width: auto;
}

.details {
  flex-basis: 100%;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.details summary {
  cursor: pointer;
}

.details dl {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.2rem 0.8rem;
  margin: 0.4rem 0 0;
}

.details dt {
  color: var(--text-faint);
}

.details dd {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 8rem;
  overflow-y: auto;
}
</style>
