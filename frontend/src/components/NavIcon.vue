<template>
  <svg class="nav-icon" viewBox="0 0 24 24" aria-hidden="true">
    <path v-for="(d, i) in ICONS[name] || []" :key="i" :d="d" />
  </svg>
</template>

<script setup>
// The sidebar's line icons, on a 24x24 grid like the built-in icons
// (builtinIcons.js). Drawn in currentColor.
defineProps({ name: { type: String, required: true } })

// A circle as a path, so every icon is just a list of paths.
const circle = (cx, cy, r) => `M${cx - r} ${cy}a${r} ${r} 0 1 0 ${2 * r} 0a${r} ${r} 0 1 0 ${-2 * r} 0`

const ICONS = {
  home: ['M3 11l9-7 9 7', 'M5 10v10h14V10', 'M10 20v-6h4v6'],
  characters: [circle(9, 7, 4), 'M2 21v-1a6 6 0 0 1 6-6h2a6 6 0 0 1 6 6v1', 'M16 3.2a4 4 0 0 1 0 7.6', 'M22 21v-1a6 6 0 0 0-4-5.6'],
  factions: ['M5 21V4', 'M5 4h12l-2.5 4 2.5 4H5'],
  chapters: ['M6.5 3H20v18H6.5A2.5 2.5 0 0 1 4 18.5v-13A2.5 2.5 0 0 1 6.5 3z', 'M4 18.5A2.5 2.5 0 0 1 6.5 16H20'],
  map: ['M9 4L3 6v14l6-2 6 2 6-2V4l-6 2-6-2z', 'M9 4v14', 'M15 6v14'],
  bestiary: [
    'M12 13c-3 0-5 3-5 5a2 2 0 0 0 2 2c1 0 2-.6 3-.6s2 .6 3 .6a2 2 0 0 0 2-2c0-2-2-5-5-5z',
    circle(5.5, 10.5, 1.6), circle(9.5, 6, 1.6), circle(14.5, 6, 1.6), circle(18.5, 10.5, 1.6),
  ],
  assets: ['M12 2l2.5 3.5V15h-5V5.5z', 'M6.5 15h11', 'M12 15v5', 'M10 21h4'],
  timeline: ['M3 12h18', circle(7, 12, 2), circle(17, 12, 2), 'M12 7v10'],
  calendar: ['M4 5h16v16H4z', 'M4 10h16', 'M8 3v4', 'M16 3v4'],
  events: ['M13 2L5 14h6l-1 8 8-12h-6z'],
  wiki: ['M12 6c-2-1.5-5-2-8-2v14c3 0 6 .5 8 2 2-1.5 5-2 8-2V4c-3 0-6 .5-8 2z', 'M12 6v14'],
  graph: [circle(5.5, 6, 2.5), circle(18.5, 6, 2.5), circle(12, 18, 2.5), 'M8 6h8', 'M7 8.2l3.8 7.6', 'M17 8.2l-3.8 7.6'],
  settings: ['M4 6h9', 'M17 6h3', 'M15 4v4', 'M4 12h3', 'M11 12h9', 'M9 10v4', 'M4 18h11', 'M19 18h1', 'M17 16v4'],
  search: [circle(11, 11, 7), 'M21 21l-5-5'],
  switch: ['M8 9l4-4 4 4', 'M8 15l4 4 4-4'],
  collapse: ['M4 4h16v16H4z', 'M9 4v16', 'M15.5 10l-2 2 2 2'],
  expand: ['M4 4h16v16H4z', 'M9 4v16', 'M13.5 10l2 2-2 2'],
  menu: ['M4 7h16', 'M4 12h16', 'M4 17h16'],
  close: ['M6 6l12 12', 'M18 6L6 18'],
}
</script>

<style scoped>
.nav-icon {
  width: 1.15rem;
  height: 1.15rem;
  flex-shrink: 0;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
