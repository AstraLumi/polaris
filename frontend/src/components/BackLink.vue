<template>
  <a :href="href" class="back-link" @click="go">{{ text }}</a>
</template>

<script setup>
// "← Back" that returns to the page you actually came from (the timeline, a
// wiki article, a filtered list…), keeping its scroll and filters. Opened
// fresh, from a bookmark or a new tab, there's nothing to go back to, so it
// links to the page's natural parent instead, under that parent's label.
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '../i18n'

const props = defineProps({
  to: { type: [String, Object], required: true }, // the parent page
  label: { type: String, required: true }, // already translated, e.g. "← Characters"
})

const route = useRoute()
const router = useRouter()

// vue-router keeps the previous in-app location in history.state.back (null
// on a fresh load). Reading route makes this recompute when the same view is
// reused for another page, such as one wiki article linking to the next.
const previous = computed(() => {
  void route.fullPath
  return router.options.history.state.back || ''
})

const parent = computed(() => router.resolve(props.to))
const toParent = computed(() => !previous.value || router.resolve(previous.value).path === parent.value.path)

const text = computed(() => (toParent.value ? props.label : t('← Back')))
const href = computed(() => (previous.value ? router.resolve(previous.value).href : parent.value.href))

function go(e) {
  if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return // new tab and friends
  e.preventDefault()
  if (previous.value) router.back()
  else router.push(props.to)
}
</script>
