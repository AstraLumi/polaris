<template>
  <div class="appears-in">
    <p v-if="error" class="faint">{{ error }}</p>
    <p v-else-if="rows === null" class="faint">{{ $t('Loading…') }}</p>
    <template v-else-if="rows.length">
      <p class="faint total">{{ $tn('Named {n} time in {c} chapter', 'Named {n} times in {c} chapters', total, { n: total, c: chapterCount }) }}</p>
      <ul class="appears-list">
        <li v-for="r in rows" :key="r.chapter_id">
          <RouterLink :to="writeLink(r)" :title="$t('Open this chapter in the editor')">
            <span v-if="r.volume_title" class="vol">{{ r.volume_title }} ·</span>
            {{ r.number }}. {{ r.title }}
          </RouterLink>
          <span class="count" :title="$tn('{n} time', '{n} times', r.count, { n: r.count })">×{{ r.count }}</span>
        </li>
      </ul>
    </template>
    <p v-else class="faint">{{ empty || $t('Not named in any chapter’s text yet. Link names as you write (Tab).') }}</p>
  </div>
</template>

<script setup>
// The chapters whose text names an item (story links, and lines spoken for
// a character), in reading order, each opening the chapter in the editor.
import { ref, computed, watch } from 'vue'
import { mentionsApi, writeLink } from '../chapterMentions'

const props = defineProps({
  type: { type: String, required: true },
  id: { type: [String, Number], required: true },
  empty: { type: String, default: '' },
})

const rows = ref(null)
const error = ref('')
const total = computed(() => (rows.value || []).reduce((n, r) => n + r.count, 0))
const chapterCount = computed(() => (rows.value || []).length)

watch(
  () => `${props.type}/${props.id}`,
  async () => {
    rows.value = null
    error.value = ''
    try {
      rows.value = await mentionsApi.appearsIn(props.type, props.id)
    } catch (e) {
      error.value = e.message
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.appears-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.2rem;
}

.appears-list li {
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
}

.appears-list a {
  min-width: 0;
}

.vol {
  color: var(--text-muted);
}

.count {
  margin-left: auto;
  font-size: 0.8rem;
  color: var(--text-faint);
  font-variant-numeric: tabular-nums;
}

.total {
  margin: 0 0 0.4rem;
  font-size: 0.85rem;
}

.faint {
  color: var(--text-faint);
}
</style>
