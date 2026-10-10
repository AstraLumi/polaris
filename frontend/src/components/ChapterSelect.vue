<template>
  <label class="field">
    <span>{{ label }}</span>
    <select :value="modelValue ?? ''" @change="$emit('update:modelValue', $event.target.value ? Number($event.target.value) : null)">
      <option value="">{{ $t('No chapter') }}</option>
      <template v-for="g in chapterGroups" :key="g.id ?? 'none'">
        <optgroup v-if="g.title" :label="g.title">
          <option v-for="c in g.chapters" :key="c.id" :value="c.id">{{ $t('Ch. {n}', { n: c.number }) }}: {{ c.title }}</option>
        </optgroup>
        <template v-else>
          <option v-for="c in g.chapters" :key="c.id" :value="c.id">{{ $t('Ch. {n}', { n: c.number }) }}: {{ c.title }}</option>
        </template>
      </template>
    </select>
  </label>
</template>

<script setup>
// Picks the chapter something happens in (an event, a character version).
import { onMounted } from 'vue'
import { chapterGroups, loadChapters } from '../chapters'

defineProps({
  modelValue: { type: Number, default: null },
  label: { type: String, required: true },
})
defineEmits(['update:modelValue'])

onMounted(() => loadChapters())
</script>
