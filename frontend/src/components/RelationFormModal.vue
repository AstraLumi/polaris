<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel panel-solid">
      <h2>{{ relation ? $t('Edit relation') : $t('New relation') }}</h2>

      <form @submit.prevent="submit">
        <label class="field">
          <span>{{ $t('With') }}</span>
          <select v-model.number="otherId" required>
            <option :value="null" disabled>{{ $t('Pick a character…') }}</option>
            <option v-for="c in others" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </label>

        <label class="field">
          <span>{{ $t('Relation') }}</span>
          <select v-model="kind">
            <option v-for="k in RELATION_KINDS" :key="k.key" :value="k.key">{{ $t(k.label) }}</option>
          </select>
        </label>

        <!-- Parent/child and mentor/student read differently from each side. -->
        <div v-if="asymmetric" class="field">
          <span>{{ $t('Which way round?') }}</span>
          <div class="side-picker">
            <button type="button" class="side-option" :class="{ 'is-on': meIsFrom }" @click="meIsFrom = true">
              {{ meName }} → {{ $t(kindDef.from) }} {{ otherName }}
            </button>
            <button type="button" class="side-option" :class="{ 'is-on': !meIsFrom }" @click="meIsFrom = false">
              {{ meName }} → {{ $t(kindDef.to) }} {{ otherName }}
            </button>
          </div>
        </div>

        <div v-if="kind === 'custom'" class="field-row">
          <label class="field">
            <span>{{ $t('How {name} relates to them', { name: meName }) }}</span>
            <input v-model="myWording" type="text" maxlength="80" :placeholder="$t('e.g. Sworn shield of')" required />
          </label>
          <label class="field">
            <span>{{ $t('How they relate to {name}', { name: meName }) }}</span>
            <input v-model="theirWording" type="text" maxlength="80" :placeholder="$t('e.g. Protected by')" />
          </label>
        </div>

        <p v-if="otherId" class="preview">
          <strong>{{ meName }}</strong>: {{ preview.mine }} {{ otherName }}<br />
          <strong>{{ otherName }}</strong>: {{ preview.theirs }} {{ meName }}
        </p>

        <div class="field-row">
          <ChapterSelect v-model="sinceChapter" :label="$t('From chapter')" />
          <ChapterSelect v-model="untilChapter" :label="$t('Until chapter')" />
        </div>
        <div class="field-row">
          <DateField v-model="since" :label="$t('Since')" />
          <DateField v-model="until" :label="$t('Until')" />
        </div>
        <p class="hint">{{ $t('All optional. Each version of a character shows the relations that hold at its own chapter (or date): friends until chapter 5 and adopted from chapter 5 is two relations.') }}</p>

        <label class="field">
          <span>{{ $t('Notes') }}</span>
          <textarea v-model="notes" rows="3"></textarea>
        </label>

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button v-if="relation" type="button" class="btn btn-danger" @click="$emit('delete', relation)">{{ $t('Delete') }}</button>
          <span class="spacer"></span>
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">
            {{ saving ? $t('Saving…') : $t('Save') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
// Adds or edits a relation between the character whose sheet this is ("me")
// and another. Stored once, read from both sides (see relations.js).
import { ref, computed } from 'vue'
import { t } from '../i18n'
import { relationsApi } from '../api'
import { RELATION_KINDS, relationKind } from '../relations'
import DateField from './DateField.vue'
import ChapterSelect from './ChapterSelect.vue'

const props = defineProps({
  me: { type: Object, required: true }, // { id, name }
  relation: { type: Object, default: null },
  characters: { type: Array, default: () => [] }, // [{ id, name }]
})
const emit = defineEmits(['close', 'saved', 'delete'])

const r = props.relation
const meIsFrom = ref(!r || r.from_id === props.me.id)
const otherId = ref(r ? (meIsFrom.value ? r.to_id : r.from_id) : null)
const kind = ref(r?.kind ?? 'friend')
// Custom wording from each side, whichever way the relation is stored.
const myWording = ref(r ? (meIsFrom.value ? r.label : r.reverse_label) : '')
const theirWording = ref(r ? (meIsFrom.value ? r.reverse_label : r.label) : '')
const since = ref(r?.since ?? '')
const until = ref(r?.until ?? '')
const sinceChapter = ref(r?.since_chapter_id ?? null)
const untilChapter = ref(r?.until_chapter_id ?? null)
const notes = ref(r?.notes ?? '')
const saving = ref(false)
const error = ref('')

const others = computed(() => props.characters.filter((c) => c.id !== props.me.id))
const meName = computed(() => props.me.name)
const otherName = computed(() => others.value.find((c) => c.id === otherId.value)?.name || '…')
const kindDef = computed(() => relationKind(kind.value))
const asymmetric = computed(() => kindDef.value && kindDef.value.from && kindDef.value.from !== kindDef.value.to)

const preview = computed(() => {
  const k = kindDef.value
  if (kind.value === 'custom') return { mine: myWording.value || '…', theirs: theirWording.value || myWording.value || '…' }
  const fromSide = t(k.from)
  const toSide = t(k.to)
  return meIsFrom.value ? { mine: fromSide, theirs: toSide } : { mine: toSide, theirs: fromSide }
})

async function submit() {
  if (!otherId.value) return
  saving.value = true
  error.value = ''
  // Everything but parent/child and mentor/student is stored from this
  // character's side (custom wording is already relative to them).
  const fromMe = asymmetric.value ? meIsFrom.value : true
  const payload = {
    from_id: fromMe ? props.me.id : otherId.value,
    to_id: fromMe ? otherId.value : props.me.id,
    kind: kind.value,
    label: kind.value === 'custom' ? myWording.value : '',
    reverse_label: kind.value === 'custom' ? theirWording.value : '',
    since: since.value,
    until: until.value,
    since_chapter_id: sinceChapter.value,
    until_chapter_id: untilChapter.value,
    notes: notes.value,
  }
  try {
    const saved = r ? await relationsApi.update(r.id, payload) : await relationsApi.create(payload)
    emit('saved', saved)
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.side-picker {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.side-option {
  text-align: left;
  padding: 0.5rem 0.7rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}

.side-option.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.preview {
  margin: 0.25rem 0 0.9rem;
  padding: 0.6rem 0.8rem;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  font-size: 0.85rem;
  color: var(--text-muted);
  line-height: 1.6;
}

.preview strong {
  color: var(--text-primary);
}

.hint {
  margin: -0.4rem 0 0.9rem;
  font-size: 0.75rem;
  color: var(--text-faint);
}
</style>
