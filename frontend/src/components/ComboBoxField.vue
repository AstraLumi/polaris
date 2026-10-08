<template>
  <label class="field">
    <span>{{ label }}</span>
    <input
      :value="modelValue"
      type="text"
      :list="listId"
      :placeholder="placeholder"
      @input="$emit('update:modelValue', $event.target.value)"
    />
    <datalist :id="listId">
      <option v-for="opt in options" :key="opt" :value="opt" />
    </datalist>
  </label>
</template>

<script setup>
defineProps({
  modelValue: { type: String, default: '' },
  label: { type: String, required: true },
  options: { type: Array, default: () => [] }, // array of plain name strings
  listId: { type: String, required: true }, // must be unique per instance on the page
  placeholder: { type: String, default: '' },
})
defineEmits(['update:modelValue'])
</script>
