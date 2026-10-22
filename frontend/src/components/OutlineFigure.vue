<template>
  <!-- A body outline standing at (x, ground), height tall, as one shape:
       every part's outline is drawn first, then every part's fill over it,
       so the parts merge. Goes inside an <svg>. -->
  <g :transform="`translate(${x} ${ground - height}) scale(${height})`" class="outline-figure">
    <path
      v-for="(d, i) in shape.shapes"
      :key="'s' + i"
      :d="d"
      fill="none"
      :stroke="stroke"
      :stroke-width="line * 2"
      stroke-linejoin="round"
      vector-effect="non-scaling-stroke"
    />
    <path v-for="(d, i) in shape.shapes" :key="'f' + i" :d="d" :fill="fill" />
  </g>
</template>

<script setup>
import { computed } from 'vue'
import { figure } from '../bodyOutlines'

const props = defineProps({
  outline: { type: Object, required: true }, // { sex, age, build }
  x: { type: Number, default: 0 },
  ground: { type: Number, default: 0 },
  height: { type: Number, required: true }, // px
  fill: { type: String, default: 'var(--surface)' }, // opaque, or the parts' outlines show through
  stroke: { type: String, default: 'var(--accent)' },
  line: { type: Number, default: 1.5 }, // px
})

const shape = computed(() => figure(props.outline))
</script>
