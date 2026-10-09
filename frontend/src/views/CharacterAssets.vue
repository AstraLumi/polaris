<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Character Assets') }}</h1>
        <p class="page-sub">{{ $t('Classes, Subclasses, Specializations, Races, Body Types, Gear and Spells live here.') }}</p>
      </div>
    </header>

    <div class="tab-bar">
      <button
        v-for="tb in tabs"
        :key="tb.key"
        class="tab-button"
        :class="{ 'is-active': tab === tb.key }"
        @click="tab = tb.key"
      >
        {{ $t(tb.label) }}
      </button>
    </div>

    <div class="bulk-bar">
      <template v-if="selectMode">
        <span class="bulk-count">{{ $tn('{n} selected', '{n} selected', selectedIds.size) }}</span>
        <button class="btn btn-danger small" :disabled="!selectedIds.size" @click="bulkConfirm = true">
          {{ $t('Delete selected ({n})', { n: selectedIds.size }) }}
        </button>
      </template>
      <button class="btn btn-ghost small" @click="toggleSelectMode">{{ selectMode ? $t('Cancel') : $t('Select several') }}</button>
    </div>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>

    <!-- Classes -->
    <section v-if="tab === 'classes'">
      <div class="section-header">
        <p class="section-hint">{{ $t('Flat (or per-level, for Base Stats) bonuses applied whenever this class is set.') }}</p>
        <button class="btn btn-primary" @click="classAdd = true">{{ $t('Add class') }}</button>
      </div>
      <AssetEmptyState v-if="!loading && classes.length === 0" kind="classes" />
      <ul class="asset-list">
        <li v-for="c in classes" :key="c.id" class="asset-row glass-panel">
          <div class="asset-name-group">
            <span class="asset-icon"><IconImage :src="c.icon_path" :name="c.name" /></span>
            <span class="asset-name">{{ c.name }}</span>
          </div>
          <div class="asset-actions">
            <RowActions kind="class" :item="c" />
          </div>
        </li>
      </ul>
    </section>

    <!-- Subclasses -->
    <section v-else-if="tab === 'subclasses'">
      <div class="section-header">
        <p class="section-hint">{{ $t('Each subclass can be restricted to one or more classes.') }}</p>
        <button class="btn btn-primary" @click="subclassAdd = true">{{ $t('Add subclass') }}</button>
      </div>
      <AssetEmptyState v-if="!loading && subclasses.length === 0" kind="subclasses" />
      <ul class="asset-list">
        <li v-for="s in subclasses" :key="s.id" class="asset-row glass-panel">
          <div class="asset-name-group">
            <span class="asset-name">{{ s.name }}</span>
            <span v-if="s.class_names.length" class="scope-badges">
              <span v-for="cn in s.class_names" :key="cn" class="scope-badge">{{ cn }}</span>
            </span>
            <span v-else class="scope-badge scope-badge-any">{{ $t('Any class') }}</span>
          </div>
          <div class="asset-actions">
            <RowActions kind="subclass" :item="s" />
          </div>
        </li>
      </ul>
    </section>

    <!-- Specializations -->
    <section v-else-if="tab === 'specializations'">
      <div class="section-header">
        <p class="section-hint">{{ $t('Each specialization belongs to exactly one class.') }}</p>
        <button class="btn btn-primary" @click="specializationAdd = true">{{ $t('Add specialization') }}</button>
      </div>
      <AssetEmptyState v-if="!loading && specializations.length === 0" kind="specializations" />
      <ul class="asset-list">
        <li v-for="s in specializations" :key="s.id" class="asset-row glass-panel">
          <div class="asset-name-group">
            <span class="asset-name">{{ s.name }}</span>
            <span class="scope-badge">{{ s.class_name }}</span>
          </div>
          <div class="asset-actions">
            <RowActions kind="specialization" :item="s" />
          </div>
        </li>
      </ul>
    </section>

    <!-- Races -->
    <section v-else-if="tab === 'races'">
      <div class="section-header">
        <p class="section-hint">{{ $t('Flat (or per-level, for Base Stats) bonuses applied whenever this race is set.') }}</p>
        <button class="btn btn-primary" @click="raceAdd = true">{{ $t('Add race') }}</button>
      </div>
      <AssetEmptyState v-if="!loading && races.length === 0" kind="races" />
      <ul class="asset-list">
        <li v-for="r in races" :key="r.id" class="asset-row glass-panel">
          <span class="asset-name">{{ r.name }}</span>
          <div class="asset-actions">
            <RowActions kind="race" :item="r" />
          </div>
        </li>
      </ul>
    </section>

    <!-- Body Types -->
    <section v-else-if="tab === 'bodyTypes'">
      <div class="section-header">
        <p class="section-hint">{{ $t('Flat (or per-level, for Base Stats) bonuses applied whenever this body type is set.') }}</p>
        <button class="btn btn-primary" @click="bodyTypeAdd = true">{{ $t('Add body type') }}</button>
      </div>
      <AssetEmptyState v-if="!loading && bodyTypes.length === 0" kind="bodyTypes" />
      <ul class="asset-list">
        <li v-for="b in bodyTypes" :key="b.id" class="asset-row glass-panel">
          <span class="asset-name">{{ b.name }}</span>
          <div class="asset-actions">
            <RowActions kind="bodyType" :item="b" />
          </div>
        </li>
      </ul>
    </section>

    <!-- Spells -->
    <section v-else-if="tab === 'spells'">
      <div class="section-header">
        <p class="section-hint">
          {{ $t('Spells are created here, then added to a character from the Spells tab on their sheet.') }}
        </p>
        <button class="btn btn-primary" @click="spellAdd = true">{{ $t('Add spell') }}</button>
      </div>
      <AssetEmptyState
        v-if="!loading && spells.length === 0"
        kind="spells"
        :hint="$t('Add one to make it available on characters.')"
      />
      <div class="spell-list">
        <SpellRow v-for="s in spells" :key="s.id" :spell="s">
          <template #actions>
            <RowActions kind="spell" :item="s" />
          </template>
        </SpellRow>
      </div>
    </section>

    <!-- Gear -->
    <section v-else-if="tab === 'gear'">
      <div class="section-header">
        <p class="section-hint">
          {{ $t('Gear gives flat stat bonuses. A piece has one slot type and fits any slot of that type, so a Glove goes in either Glove slot.') }}
        </p>
        <button class="btn btn-primary" @click="gearAdd = true">{{ $t('Add gear') }}</button>
      </div>
      <div v-if="gearList.length" class="slot-filter">
        <button class="chip" :class="{ 'is-active': gearFilter === '' }" @click="gearFilter = ''">{{ $t('All') }}</button>
        <button
          v-for="st in usedSlotTypes"
          :key="st.id"
          class="chip"
          :class="{ 'is-active': gearFilter === st.id }"
          @click="gearFilter = st.id"
        >{{ $t(st.label) }}</button>
      </div>
      <AssetEmptyState
        v-if="!loading && gearList.length === 0"
        kind="gear"
        :hint="$t('Add one to make it available on characters.')"
      />
      <div class="spell-list">
        <GearRow v-for="g in filteredGear" :key="g.id" :gear="g">
          <template #actions>
            <RowActions kind="gear" :item="g" />
          </template>
        </GearRow>
      </div>
    </section>

    <!-- Modals: simple assets share one modal component -->
    <SimpleAssetFormModal
      v-if="classAdd || classEdit"
      kind="class"
      with-icon
      :api="classesApi"
      :asset="classEdit"
      @close="classAdd = false; classEdit = null"
      @saved="onSaved('classes')"
    />
    <SimpleAssetFormModal
      v-if="raceAdd || raceEdit"
      kind="race"
      :api="racesApi"
      :asset="raceEdit"
      @close="raceAdd = false; raceEdit = null"
      @saved="onSaved('races')"
    />
    <SimpleAssetFormModal
      v-if="bodyTypeAdd || bodyTypeEdit"
      kind="body type"
      :api="bodyTypesApi"
      :asset="bodyTypeEdit"
      @close="bodyTypeAdd = false; bodyTypeEdit = null"
      @saved="onSaved('bodyTypes')"
    />
    <SpecializationFormModal
      v-if="specializationAdd || specializationEdit"
      :classes="classes"
      :specialization="specializationEdit"
      @close="specializationAdd = false; specializationEdit = null"
      @saved="onSaved('specializations')"
    />
    <SubclassFormModal
      v-if="subclassAdd || subclassEdit"
      :classes="classes"
      :subclass="subclassEdit"
      @close="subclassAdd = false; subclassEdit = null"
      @saved="onSaved('subclasses')"
    />

    <SpellFormModal
      v-if="spellAdd || spellEdit"
      :catalogs="spellCatalogs"
      :spell="spellEdit"
      @close="spellAdd = false; spellEdit = null"
      @saved="onSaved"
    />

    <GearFormModal
      v-if="gearAdd || gearEdit"
      :gear="gearEdit"
      @close="gearAdd = false; gearEdit = null"
      @saved="onSaved"
    />

    <ConfirmDialog
      v-if="bulkConfirm"
      :title="$tn('Delete {n} item?', 'Delete {n} items?', selectedIds.size)"
      :message="bulkMessage"
      :confirm-label="$t('Delete')"
      @cancel="bulkConfirm = false"
      @confirm="handleBulkDelete"
    />

    <ConfirmDialog
      v-if="confirmTarget"
      :title="$t('Delete {name}?', { name: confirmTarget.item.name })"
      :message="confirmMessage"
      :confirm-label="$t('Delete')"
      @cancel="confirmTarget = null"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, h, onMounted } from 'vue'
import { t, tn, tr } from '../i18n'
import {
  fetchCatalogs, classesApi, racesApi, bodyTypesApi, specializationsApi, subclassesApi, spellsApi, gearApi,
} from '../api'
import SimpleAssetFormModal from '../components/SimpleAssetFormModal.vue'
import IconImage from '../components/IconImage.vue'
import SpecializationFormModal from '../components/SpecializationFormModal.vue'
import SubclassFormModal from '../components/SubclassFormModal.vue'
import SpellFormModal from '../components/SpellFormModal.vue'
import SpellRow from '../components/SpellRow.vue'
import GearRow from '../components/GearRow.vue'
import GearFormModal from '../components/GearFormModal.vue'
import { GEAR_SLOT_TYPES } from '../gear'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const tabs = [
  { key: 'classes', label: tr('Classes') },
  { key: 'subclasses', label: tr('Subclasses') },
  { key: 'specializations', label: tr('Specializations') },
  { key: 'races', label: tr('Races') },
  { key: 'bodyTypes', label: tr('Body Types') },
  { key: 'gear', label: tr('Gear') },
  { key: 'spells', label: tr('Spells') },
]
const tab = ref('classes')

const AssetEmptyState = {
  props: {
    kind: String,
    hint: { type: String, default: '' },
  },
  render() {
    const titles = {
      classes: () => t('No classes yet.'),
      subclasses: () => t('No subclasses yet.'),
      specializations: () => t('No specializations yet.'),
      races: () => t('No races yet.'),
      bodyTypes: () => t('No body types yet.'),
      spells: () => t('No spells yet.'),
      gear: () => t('No gear yet.'),
    }
    return h('div', { class: 'empty-state glass-panel' }, [
      h('p', { class: 'empty-title' }, titles[this.kind]()),
      h('p', { class: 'empty-hint' }, this.hint || t('Add one to start attaching stat bonuses to it.')),
    ])
  },
}

const loading = ref(true)
const loadError = ref('')

const classes = ref([])
const subclasses = ref([])
const specializations = ref([])
const races = ref([])
const bodyTypes = ref([])
const locations = ref([])
const spells = ref([])
const gearList = ref([])
const gearFilter = ref('')
const gearAdd = ref(false)
const gearEdit = ref(null)

const classAdd = ref(false)
const classEdit = ref(null)
const raceAdd = ref(false)
const raceEdit = ref(null)
const bodyTypeAdd = ref(false)
const bodyTypeEdit = ref(null)
const specializationAdd = ref(false)
const specializationEdit = ref(null)
const subclassAdd = ref(false)
const subclassEdit = ref(null)
const spellAdd = ref(false)
const spellEdit = ref(null)

const confirmTarget = ref(null) // { kind, item }

const spellCatalogs = computed(() => ({
  classes: classes.value,
  subclasses: subclasses.value,
  specializations: specializations.value,
  locations: locations.value,
}))

const usedSlotTypes = computed(() => GEAR_SLOT_TYPES.filter((st) => gearList.value.some((g) => g.slot === st.id)))
const filteredGear = computed(() => (gearFilter.value ? gearList.value.filter((g) => g.slot === gearFilter.value) : gearList.value))

const confirmMessage = computed(() =>
  ['spell', 'gear'].includes(confirmTarget.value?.kind)
    ? t("It's removed from every character that has it.")
    : t('Characters using it keep their data, but lose the link and any bonuses it gave them.'),
)

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const [catalogs, spellList, gl] = await Promise.all([fetchCatalogs(), spellsApi.list(), gearApi.list()])
    classes.value = catalogs.classes
    subclasses.value = catalogs.subclasses
    specializations.value = catalogs.specializations
    races.value = catalogs.races
    bodyTypes.value = catalogs.bodyTypes
    locations.value = catalogs.locations
    spells.value = spellList
    gearList.value = gl
    if (gearFilter.value && !gl.some((g) => g.slot === gearFilter.value)) gearFilter.value = ''
  } catch (err) {
    loadError.value = t("Couldn't load Character Assets. Try refreshing.")
  } finally {
    loading.value = false
  }
}

function onSaved() {
  spellAdd.value = false
  spellEdit.value = null
  gearAdd.value = false
  gearEdit.value = null
  classAdd.value = false
  classEdit.value = null
  raceAdd.value = false
  raceEdit.value = null
  bodyTypeAdd.value = false
  bodyTypeEdit.value = null
  specializationAdd.value = false
  specializationEdit.value = null
  subclassAdd.value = false
  subclassEdit.value = null
  load()
}

async function editClass(c) {
  try {
    classEdit.value = await classesApi.fetch(c.id)
  } catch (err) {
    loadError.value = t("Couldn't load that class's details.")
  }
}
async function editRace(r) {
  try {
    raceEdit.value = await racesApi.fetch(r.id)
  } catch (err) {
    loadError.value = t("Couldn't load that race's details.")
  }
}
async function editBodyType(b) {
  try {
    bodyTypeEdit.value = await bodyTypesApi.fetch(b.id)
  } catch (err) {
    loadError.value = t("Couldn't load that body type's details.")
  }
}
async function editSpecialization(s) {
  try {
    specializationEdit.value = await specializationsApi.fetch(s.id)
  } catch (err) {
    loadError.value = t("Couldn't load that specialization's details.")
  }
}
async function editSubclass(s) {
  try {
    subclassEdit.value = await subclassesApi.fetch(s.id)
  } catch (err) {
    loadError.value = t("Couldn't load that subclass's details.")
  }
}

function confirmDelete(kind, item) {
  confirmTarget.value = { kind, item }
}

// ---- Selecting several to delete at once (on the open tab) -------------------

const TAB_KIND = {
  classes: 'class', subclasses: 'subclass', specializations: 'specialization', races: 'race',
  bodyTypes: 'bodyType', gear: 'gear', spells: 'spell',
}
const selectMode = ref(false)
const selectedIds = ref(new Set())
const bulkConfirm = ref(false)

function toggleSelectMode() {
  selectMode.value = !selectMode.value
  selectedIds.value = new Set()
}
function toggleSelected(id) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}
watch(tab, () => {
  selectMode.value = false
  selectedIds.value = new Set()
})

const bulkMessage = computed(() =>
  ['spell', 'gear'].includes(TAB_KIND[tab.value])
    ? t("They're removed from every character that has them.")
    : t('Characters using them keep their data, but lose the link and any bonuses they gave them.'),
)

async function handleBulkDelete() {
  const kind = TAB_KIND[tab.value]
  bulkConfirm.value = false
  let failed = 0
  for (const id of selectedIds.value) {
    try {
      await deleteApis[kind].delete(id)
    } catch (err) {
      failed++
    }
  }
  if (failed) loadError.value = tn("Couldn't delete {n} of them.", "Couldn't delete {n} of them.", failed)
  selectMode.value = false
  selectedIds.value = new Set()
  await load()
}

const editors = {
  class: editClass, subclass: editSubclass, specialization: editSpecialization, race: editRace, bodyType: editBodyType,
  spell: (s) => (spellEdit.value = s),
  gear: (g) => (gearEdit.value = g),
}

// A row's Edit and Delete buttons, or its checkbox while selecting.
const RowActions = {
  props: { kind: String, item: Object },
  setup(p) {
    return () =>
      selectMode.value
        ? h('label', { class: 'row-pick', title: t('Select') }, [
            h('input', {
              type: 'checkbox',
              checked: selectedIds.value.has(p.item.id),
              'aria-label': p.item.name,
              onChange: () => toggleSelected(p.item.id),
            }),
          ])
        : [
            h('button', { class: 'btn btn-ghost small', onClick: () => editors[p.kind](p.item) }, t('Edit')),
            h('button', { class: 'btn btn-ghost small', onClick: () => confirmDelete(p.kind, p.item) }, t('Delete')),
          ]
  },
}

const deleteApis = {
  class: classesApi, race: racesApi, bodyType: bodyTypesApi,
  specialization: specializationsApi, subclass: subclassesApi, spell: spellsApi, gear: gearApi,
}

const deleteFailed = {
  class: () => t("Couldn't delete that class."),
  race: () => t("Couldn't delete that race."),
  bodyType: () => t("Couldn't delete that body type."),
  specialization: () => t("Couldn't delete that specialization."),
  subclass: () => t("Couldn't delete that subclass."),
  spell: () => t("Couldn't delete that spell."),
  gear: () => t("Couldn't delete that gear."),
}

async function handleDelete() {
  const { kind, item } = confirmTarget.value
  try {
    await deleteApis[kind].delete(item.id)
    confirmTarget.value = null
    await load()
  } catch (err) {
    loadError.value = deleteFailed[kind]()
    confirmTarget.value = null
  }
}

onMounted(load)
</script>

<style scoped>
.bulk-bar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 0.6rem;
  margin: -0.5rem 0 1rem;
}

.bulk-count {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.row-pick {
  display: flex;
  align-items: center;
  padding: 0.3rem 0.5rem;
  cursor: pointer;
}

.row-pick input {
  width: 1.1rem;
  height: 1.1rem;
  accent-color: var(--accent);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
}

.section-hint {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted);
  max-width: 46ch;
  line-height: 1.5;
}

.asset-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.spell-list {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.asset-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.85rem 1.1rem;
  gap: 1rem;
}

.asset-name-group {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.asset-name {
  font-family: 'Fraunces', serif;
  font-size: 1rem;
  color: var(--text-primary);
}

.scope-badges {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.scope-badge {
  font-size: 0.7rem;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  padding: 0.15rem 0.6rem;
}

.scope-badge-any {
  color: var(--text-faint);
  font-style: italic;
}

.asset-icon {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 9px;
  background: var(--accent-soft);
  font-size: 0.85rem;
}

.asset-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}

.slot-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: -0.5rem 0 1.1rem;
}

.chip {
  font: inherit;
  font-size: 0.78rem;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  padding: 0.25rem 0.8rem;
  cursor: pointer;
}

.chip:hover {
  color: var(--text-primary);
}

.chip.is-active {
  color: var(--accent);
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 40%, transparent);
}
</style>
