<template>
  <!-- A file box in the app's own look, instead of the browser's "Choose
       File / No file chosen": a button, the picked file's name, and room to
       drop a file on it. -->
  <div
    class="file-pick"
    :class="{ 'is-over': over }"
    @dragenter.prevent="over = true"
    @dragover.prevent="over = true"
    @dragleave="onLeave"
    @drop.prevent="onDrop"
  >
    <button type="button" class="btn btn-ghost small" @click="input?.click()">{{ label || $t('Choose a picture') }}</button>
    <span class="file-name">{{ fileName || $t('or drop one here') }}</span>
    <input ref="input" type="file" :accept="accept" :multiple="multiple" hidden @change="onChange" />
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { tn } from '../i18n'

// Emits `change` with the real input's event, as a plain file input would,
// so handlers keep reading e.target.files[0] (before clearing the input,
// see dropFile(keepInput)). A parent clearing it with `ref.value = ''`
// clears the real input.
defineProps({
  accept: { type: String, default: 'image/*' },
  label: { type: String, default: '' }, // already translated
  multiple: { type: Boolean, default: false }, // several files (e.target.files)
})
const emit = defineEmits(['change'])

const input = ref(null)
const over = ref(false)
const fileName = ref('')

function onChange(e) {
  const files = e.target.files
  fileName.value = files?.length > 1 ? tn('{n} file', '{n} files', files.length, { n: files.length }) : files?.[0]?.name || ''
  emit('change', e)
}

function onLeave(e) {
  if (!e.currentTarget.contains(e.relatedTarget)) over.value = false
}

// A dropped file goes into the real input, so it behaves as if picked.
function onDrop(e) {
  over.value = false
  const files = e.dataTransfer?.files
  if (!files?.length || !input.value) return
  input.value.files = files
  onChange({ target: input.value })
}

defineExpose({
  get value() {
    return input.value?.value ?? ''
  },
  set value(v) {
    if (input.value) input.value.value = v
    if (!v) fileName.value = ''
  },
  click: () => input.value?.click(),
})
</script>

<style scoped>
.file-pick {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.6rem;
  padding: 0.45rem;
  border: 1px dashed var(--glass-border);
  border-radius: var(--radius-control);
  background: rgba(255, 255, 255, 0.02);
  transition: border-color 0.12s ease, background 0.12s ease;
}

.file-pick.is-over {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.file-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.8rem;
  color: var(--text-faint);
}
</style>
