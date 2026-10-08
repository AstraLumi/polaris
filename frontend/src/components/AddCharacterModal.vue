<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel">
      <h2>{{ $t('Add a character') }}</h2>
      <form @submit.prevent="submit">
        <label class="field">
          <span>{{ $t('Name') }}</span>
          <input v-model="form.name" type="text" required autofocus />
        </label>
        <label class="field">
          <span>{{ $t('Nickname') }}</span>
          <input v-model="form.nickname" type="text" />
        </label>
        <div class="field-row">
          <label class="field">
            <span>{{ $t('Level') }}</span>
            <input v-model.number="form.level" type="number" min="1" />
          </label>
          <ComboBoxField
            v-model="form.className"
            :label="$t('Class')"
            :options="catalogs.classes.map((c) => c.name)"
            list-id="add-char-classes"
            :placeholder="$t('e.g. Warrior')"
          />
        </div>
        <div class="field-row">
          <ComboBoxField
            v-model="form.subclassName"
            :label="$t('Subclass')"
            :options="availableSubclasses.map((c) => c.name)"
            list-id="add-char-subclasses"
            :placeholder="$t('e.g. Berserker')"
          />
          <ComboBoxField
            v-model="form.specializationName"
            :label="$t('Specialization')"
            :options="catalogs.specializations.map((c) => c.name)"
            list-id="add-char-specializations"
            :placeholder="$t('e.g. Spellslinger')"
          />
        </div>
        <div class="field-row">
          <DateField v-model="form.versionDate" :label="$t('Story date (optional)')" />
          <label class="field">
            <span>{{ $t('Volume / Chapter / Page (optional)') }}</span>
            <input v-model="form.versionReference" type="text" :placeholder="$t('Vol. 1 - Ch. 1')" />
          </label>
        </div>
        <label class="field">
          <span>{{ $t('Picture') }}</span>
          <input type="file" accept="image/*" @change="onFileChange" />
        </label>

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? $t('Saving…') : $t('Save character') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { t } from '../i18n'
import { createCharacter, fetchCatalogs } from '../api'
import ComboBoxField from './ComboBoxField.vue'
import DateField from './DateField.vue'

const emit = defineEmits(['close', 'created'])

const form = reactive({
  name: '',
  nickname: '',
  level: 1,
  className: '',
  subclassName: '',
  specializationName: '',
  versionDate: '',
  versionReference: '',
})
const picture = ref(null)
const submitting = ref(false)
const error = ref('')
const catalogs = ref({ classes: [], subclasses: [], specializations: [], races: [], bodyTypes: [] })

const availableSubclasses = computed(() => {
  const typedClass = (form.className || '').trim().toLowerCase()
  return catalogs.value.subclasses.filter((s) => {
    const names = s.class_names || []
    return names.length === 0 || names.some((n) => n.trim().toLowerCase() === typedClass)
  })
})

onMounted(async () => {
  catalogs.value = await fetchCatalogs()
})

function onFileChange(e) {
  picture.value = e.target.files[0] || null
}

async function submit() {
  if (!form.name.trim()) return
  submitting.value = true
  error.value = ''
  try {
    await createCharacter({ ...form, picture: picture.value })
    emit('created')
  } catch (err) {
    error.value = t("Couldn't save that character. Try again.")
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
</style>
