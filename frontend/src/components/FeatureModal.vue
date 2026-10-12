<template>
  <div class="overlay" @click.self="$emit('close')">
    <form class="modal is-small glass-panel panel-solid" @submit.prevent="save">
      <h2>{{ feature?.id ? $t('Edit {name}', { name: feature.name }) : $t('New {kind}', { kind: $t(layer.one).toLowerCase() }) }}</h2>
      <label class="field">
        <span>{{ $t('Name') }}</span>
        <input ref="nameEl" v-model="form.name" type="text" maxlength="80" required />
      </label>
      <div class="field-row">
        <label class="field">
          <span>{{ $t('Colour') }}</span>
          <input v-model="form.color" type="color" class="color-input" />
        </label>
        <label v-if="layer.value" class="field">
          <span>{{ $t(layer.value) }}</span>
          <input v-model.number="form.value" type="number" step="any" />
        </label>
      </div>
      <RouterLink v-if="feature?.id" :to="wikiPath('map_feature', feature.id)" class="wiki-link">{{ $t('Wiki article') }} ›</RouterLink>
      <p v-if="error" class="error-banner">{{ error }}</p>
      <div class="modal-actions">
        <button v-if="feature?.id" type="button" class="btn btn-ghost danger-text" :disabled="busy" @click="confirming = true">{{ $t('Delete') }}</button>
        <span class="spacer"></span>
        <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
        <button type="submit" class="btn btn-primary" :disabled="busy || !form.name.trim()">{{ $t('Save') }}</button>
      </div>
    </form>

    <ConfirmDialog
      v-if="confirming"
      :title="$t('Delete {name}?', { name: feature.name })"
      :message="$t('Its hexes on the layer are cleared, and its wiki article goes with it.')"
      :confirm-label="$t('Delete')"
      @cancel="confirming = false"
      @confirm="remove"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { t } from '../i18n'
import { layerOf } from '../planetLayers'
import { wikiPath } from '../wiki'
import ConfirmDialog from './ConfirmDialog.vue'

// A plate, mountain range or ocean depth: its name, colour and (ranges and
// depths) a height or depth. save(feature) and remove(feature) come from
// the layers (planetLayers.js) and throw on failure.
const props = defineProps({
  layerId: { type: String, required: true },
  feature: { type: Object, default: null },
  save: { type: Function, required: true },
  remove: { type: Function, required: true },
})
const emit = defineEmits(['close', 'saved', 'deleted'])

const layer = computed(() => layerOf(props.layerId))
const PALETTE = ['#c0603a', '#5a8fd0', '#7fb069', '#d6a447', '#9b6fc4', '#4fb3a9', '#d0607e', '#8c8c8c']
const form = reactive({
  name: props.feature?.name ?? '',
  color: props.feature?.color ?? PALETTE[Math.floor(Math.random() * PALETTE.length)],
  value: props.feature?.value ?? '',
})
const busy = ref(false)
const error = ref('')
const confirming = ref(false)
const nameEl = ref(null)
onMounted(() => nameEl.value?.focus())

async function save() {
  busy.value = true
  error.value = ''
  try {
    const value = form.value === '' || form.value == null ? null : Number(form.value)
    const saved = await props.save({ id: props.feature?.id, layer: props.layerId, name: form.name.trim(), color: form.color, value })
    emit('saved', saved)
  } catch (err) {
    error.value = err.message || t("Couldn't save. Try again.")
  } finally {
    busy.value = false
  }
}

async function remove() {
  confirming.value = false
  busy.value = true
  try {
    await props.remove(props.feature)
    emit('deleted')
  } catch (err) {
    error.value = err.message || t("Couldn't delete it.")
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.overlay {
  z-index: 60;
}

.color-input {
  height: var(--control-h);
  padding: 0.15rem;
}

.wiki-link {
  font-size: 0.78rem;
  color: var(--accent);
}

.spacer {
  flex: 1;
}
</style>
