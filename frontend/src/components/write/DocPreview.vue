<template>
  <!-- A chapter's text, read only, drawn like the page (versions, the two
       texts of a conflict). -->
  <div ref="el" class="write-doc doc-preview" :class="{ 'is-online': online }"></div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { DOMSerializer } from 'prosemirror-model'
import { schema, docFromJSON } from '../../editor/schema.js'

const props = defineProps({
  doc: { type: Object, default: null }, // stored JSON
  online: { type: Boolean, default: false },
})

const el = ref(null)
const serializer = DOMSerializer.fromSchema(schema)

function draw() {
  if (!el.value) return
  el.value.replaceChildren(serializer.serializeFragment(docFromJSON(props.doc).content))
}

onMounted(draw)
watch(() => props.doc, draw)
</script>
