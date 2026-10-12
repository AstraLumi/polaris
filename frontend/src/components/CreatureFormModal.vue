<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal is-wide glass-panel panel-solid">
      <h2>{{ creature ? $t('Edit creature') : $t('New creature') }}</h2>
      <form @submit.prevent="submit">
        <div class="top">
          <div class="picture">
            <div class="picture-box">
              <img v-if="previewUrl" :src="previewUrl" alt="" />
              <span v-else>{{ (name || '?').slice(0, 1).toUpperCase() }}</span>
            </div>
            <label class="btn btn-ghost small">
              {{ $t('Picture') }}
              <input type="file" accept="image/*" hidden @change="onFile" />
            </label>
            <button v-if="previewUrl" type="button" class="btn btn-ghost small" @click="clearPicture">{{ $t('Remove') }}</button>
          </div>
          <div class="top-fields">
            <label class="field">
              <span>{{ $t('Name') }}</span>
              <input v-model="name" type="text" maxlength="120" required autofocus />
            </label>
            <div class="field">
              <span>{{ $t('Tag') }}</span>
              <div class="kind-picker" role="radiogroup" :aria-label="$t('Tag')">
                <button
                  v-for="k in CREATURE_KINDS"
                  :key="k.id"
                  type="button"
                  role="radio"
                  class="kind-option"
                  :class="['is-' + k.id, { 'is-on': kind === k.id }]"
                  :aria-checked="kind === k.id"
                  @click="kind = k.id"
                >
                  {{ $t(k.label) }}
                </button>
              </div>
            </div>
            <div class="field-row">
              <label class="field">
                <span>{{ $t('Lowest level') }}</span>
                <input v-model="levelMin" type="number" min="0" step="1" />
              </label>
              <label class="field">
                <span>{{ $t('Highest level') }}</span>
                <input v-model="levelMax" type="number" min="0" step="1" />
              </label>
            </div>
            <LocationSelect v-model="regionId" :label="$t('Native to')" :options="locationOptions" />
          </div>
        </div>
        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="description" rows="4"></textarea>
        </label>
        <div class="field">
          <span>{{ $t('Territory') }}</span>
          <p class="hint">
            {{ customTerritory
              ? $t('Painted by hand. Click or drag over hexes to add or remove them.')
              : regionId != null
                ? $t('The area of the place it is native to. Click or drag over hexes to paint its own territory instead.')
                : $t('Pick where it is native to, or click or drag over hexes to paint its territory.') }}
          </p>
          <PlanetTerritory
            v-if="onPlanet"
            v-model:cells="cellsModel"
            :land-cells="planetLand"
            :frequency="worldMap.planet.frequency"
            :kind="kind"
            editable
            :max-height="340"
          />
          <HexTerritory v-else v-model:territory="territoryModel" :map-hexes="mapHexes" :kind="kind" editable :width="560" :max-height="320" />
          <div class="territory-actions">
            <span class="hint">{{ $tn('{n} hex', '{n} hexes', onPlanet ? cells.length : territory.length) }}</span>
            <button v-if="customTerritory && regionId != null" type="button" class="btn btn-ghost small" @click="followRegion">
              {{ $t('Back to the area of the place') }}
            </button>
          </div>
        </div>
        <div class="field">
          <span>{{ $t('Related creatures') }}</span>
          <ul v-if="related.length" class="related">
            <li v-for="id in related" :key="id" class="related-chip">
              {{ nameOf(id) }}
              <button type="button" :aria-label="$t('Remove')" :title="$t('Remove')" @click="related = related.filter((x) => x !== id)">✕</button>
            </li>
          </ul>
          <select :value="''" @change="addRelated($event)">
            <option value="">{{ $t('Add a related creature…') }}</option>
            <option v-for="c in relatedOptions" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
        <p v-if="error" class="error-banner">{{ error }}</p>
        <div class="modal-actions">
          <button v-if="creature" type="button" class="btn btn-danger" @click="confirmOpen = true">{{ $t('Delete') }}</button>
          <span class="spacer"></span>
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">
            {{ saving ? $t('Saving…') : $t('Save') }}
          </button>
        </div>
      </form>
    </div>
    <Teleport to="body">
      <ConfirmDialog
        v-if="confirmOpen"
        :title="$t('Delete {name}?', { name: creature?.name })"
        :message="$t('Its wiki article goes too, and it stops being related to other creatures.')"
        :confirm-label="$t('Delete')"
        @cancel="confirmOpen = false"
        @confirm="remove"
      />
    </Teleport>
  </div>
</template>

<script setup>
// Creates or edits a creature of the bestiary, its relations included.
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { creaturesApi, fetchLocationOptions, mapApi, planetApi } from '../api'
import { CREATURE_KINDS, mapHexesOf, areaOf, planetLandOf, planetAreaOf } from '../creatures'
import { worldMap } from '../worldMap'
import LocationSelect from './LocationSelect.vue'
import HexTerritory from './HexTerritory.vue'
import PlanetTerritory from './PlanetTerritory.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const props = defineProps({
  creature: { type: Object, default: null }, // as listed, when editing
  allCreatures: { type: Array, default: () => [] },
})
const emit = defineEmits(['close', 'saved', 'deleted'])

const c = props.creature
const name = ref(c?.name ?? '')
const kind = ref(c?.kind ?? 'monster')
const levelMin = ref(c?.level_min ?? '')
const levelMax = ref(c?.level_max ?? '')
const regionId = ref(c?.region_id ?? null)
const description = ref(c?.description ?? '')
const related = ref((c?.related ?? []).map((r) => r.id))

// The territory follows the place it's native to until a hex is painted;
// picking another place goes back to following.
const map = ref(null)
const mapHexes = computed(() => mapHexesOf(map.value))
const territory = ref(c?.territory ?? [])
const customTerritory = ref(!!c?.custom_territory)
const territoryModel = computed({
  get: () => territory.value,
  set: (hexes) => {
    territory.value = hexes
    customTerritory.value = true
  },
})
// On a planet (Settings → Map) the territory is cells, painted on a globe.
const onPlanet = computed(() => worldMap.mode === '3d')
const planet = ref(null)
const planetLand = computed(() => planetLandOf(planet.value))
const cells = ref(c?.cells ?? [])
const cellsModel = computed({
  get: () => cells.value,
  set: (list) => {
    cells.value = list
    customTerritory.value = true
  },
})
function followRegion() {
  customTerritory.value = false
  territory.value = areaOf(map.value, regionId.value)
  cells.value = planetAreaOf(planet.value, regionId.value)
}
watch(regionId, followRegion)
const locationOptions = ref([])
const saving = ref(false)
const error = ref('')
const confirmOpen = ref(false)

const pictureFile = ref(null)
const removePicture = ref(false)
let objectUrl = ''
const previewUrl = computed(() => (pictureFile.value ? objectUrl : removePicture.value ? '' : c?.picture_path || ''))

const relatedOptions = computed(() => props.allCreatures.filter((x) => x.id !== c?.id && !related.value.includes(x.id)))
const nameOf = (id) => props.allCreatures.find((x) => x.id === id)?.name || '…'

function addRelated(e) {
  const id = Number(e.target.value)
  e.target.value = ''
  if (id && !related.value.includes(id)) related.value = [...related.value, id]
}

function onFile(e) {
  const file = e.target.files[0] || null
  e.target.value = ''
  if (!file) return
  if (objectUrl) URL.revokeObjectURL(objectUrl)
  pictureFile.value = file
  objectUrl = URL.createObjectURL(file)
  removePicture.value = false
}

function clearPicture() {
  if (objectUrl) URL.revokeObjectURL(objectUrl)
  objectUrl = ''
  pictureFile.value = null
  removePicture.value = true
}

onBeforeUnmount(() => objectUrl && URL.revokeObjectURL(objectUrl))

async function submit() {
  saving.value = true
  error.value = ''
  const fd = new FormData()
  fd.append('name', name.value)
  fd.append('kind', kind.value)
  fd.append('level_min', levelMin.value ?? '')
  fd.append('level_max', levelMax.value ?? '')
  fd.append('region_id', regionId.value ?? '')
  fd.append('description', description.value)
  fd.append('related', JSON.stringify(related.value))
  if (!customTerritory.value) fd.append('territory', 'region')
  else if (onPlanet.value) fd.append('territory_cells', JSON.stringify(cells.value))
  else fd.append('territory', JSON.stringify(territory.value))
  if (pictureFile.value) fd.append('picture', pictureFile.value)
  if (removePicture.value) fd.append('remove_picture', '1')
  try {
    emit('saved', await creaturesApi.save(c?.id, fd))
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove() {
  try {
    await creaturesApi.remove(c.id)
    confirmOpen.value = false
    emit('deleted')
  } catch (e) {
    error.value = e.message
    confirmOpen.value = false
  }
}

onMounted(async () => {
  ;[locationOptions.value, map.value, planet.value] = await Promise.all([
    fetchLocationOptions(),
    mapApi.get().catch(() => null),
    onPlanet.value ? planetApi.get().catch(() => null) : null,
  ])
})
</script>

<style scoped>
.top {
  display: flex;
  gap: 1.25rem;
  align-items: flex-start;
}

.picture {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
}

.picture-box {
  width: 6rem;
  height: 6rem;
  border-radius: 14px;
  border: 2px solid var(--glass-border);
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'Fraunces', serif;
  font-size: 2rem;
  color: var(--accent);
}

.picture-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.top-fields {
  flex: 1;
  min-width: 0;
}

.kind-picker {
  display: flex;
  gap: 0.35rem;
}

.kind-option {
  flex: 1;
  min-height: var(--control-h);
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}

.kind-option.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.related {
  list-style: none;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin: 0 0 0.5rem;
  padding: 0;
}

.related-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.2rem 0.4rem 0.2rem 0.65rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  font-size: 0.82rem;
  color: var(--text-primary);
}

.related-chip button {
  background: none;
  border: none;
  color: var(--text-faint);
  cursor: pointer;
  font-size: 0.75rem;
}

.related-chip button:hover {
  color: var(--danger-text);
}

.spacer {
  flex: 1;
}

.hint {
  margin: 0 0 0.4rem;
  font-size: 0.78rem;
  color: var(--text-faint);
}

.territory-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 0.4rem;
}

@media (max-width: 720px) {
  .top {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
