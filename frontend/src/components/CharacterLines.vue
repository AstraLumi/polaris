<template>
  <div class="char-lines">
    <p v-if="error" class="error-banner">{{ error }}</p>
    <p v-else-if="lines === null" class="faint">{{ $t('Loading…') }}</p>
    <template v-else-if="lines.length">
      <div class="lines-tools no-print">
        <input v-model="query" type="search" class="lines-search" :placeholder="$t('Search their lines')" :aria-label="$t('Search their lines')" />
        <select v-model="chapterFilter" :aria-label="$t('Chapter')">
          <option value="">{{ $tn('All chapters ({n} line)', 'All chapters ({n} lines)', lines.length, { n: lines.length }) }}</option>
          <option v-for="c in chapters" :key="c.chapter_id" :value="String(c.chapter_id)">{{ chapterName(c) }} ({{ c.count }})</option>
        </select>
      </div>
      <p v-if="!groups.length" class="faint">{{ $t('No line matches.') }}</p>
      <section v-for="g in groups" :key="g.chapter_id" class="lines-chapter">
        <h3>
          <RouterLink :to="writeLink(g)">{{ chapterName(g) }}</RouterLink>
          <span class="faint">{{ $tn('{n} line', '{n} lines', g.lines.length, { n: g.lines.length }) }}</span>
        </h3>
        <ol class="lines-list">
          <li v-for="l in g.lines" :key="l.position">
            <RouterLink :to="writeLink(l, l.position)" class="line" :title="$t('Open this line in the editor')">
              <span v-if="l.manner" class="manner">{{ l.manner }}</span>
              <span class="said">{{ l.text || '…' }}</span>
              <span v-if="l.speaker && l.speaker !== name" class="as faint">{{ $t('as “{name}”', { name: l.speaker }) }}</span>
            </RouterLink>
          </li>
        </ol>
      </section>
    </template>
    <p v-else class="faint">{{ $t('No lines yet. In the editor, Script Mode turns “Name: …” into a line said by that character.') }}</p>
  </div>
</template>

<script setup>
// Everything a character says in the book (the dialogue lines that name
// them as the speaker, backend/chapterindex.go), in reading order, with a
// search and a chapter filter. Each line opens in the editor (?line=).
import { ref, computed, watch } from 'vue'
import { mentionsApi, writeLink } from '../chapterMentions'
import { fold } from '../editor/nameMatch.js'

const props = defineProps({
  characterId: { type: [String, Number], required: true },
  name: { type: String, default: '' },
})

const lines = ref(null)
const error = ref('')
const query = ref('')
const chapterFilter = ref('')

watch(
  () => props.characterId,
  async (id) => {
    lines.value = null
    error.value = ''
    chapterFilter.value = ''
    try {
      lines.value = await mentionsApi.lines(id)
    } catch (e) {
      error.value = e.message
    }
  },
  { immediate: true },
)

const chapterName = (c) => (c.volume_title ? `${c.volume_title} · ` : '') + (c.number ? `${c.number}. ` : '') + c.title

// The chapters they speak in, with how many lines.
const chapters = computed(() => {
  const out = []
  for (const l of lines.value || []) {
    const last = out[out.length - 1]
    if (last && last.chapter_id === l.chapter_id) last.count++
    else out.push({ ...l, count: 1 })
  }
  return out
})

const groups = computed(() => {
  const q = fold(query.value.trim())
  const out = []
  for (const l of lines.value || []) {
    if (chapterFilter.value && String(l.chapter_id) !== chapterFilter.value) continue
    if (q && !fold(`${l.manner} ${l.text}`).includes(q)) continue
    const last = out[out.length - 1]
    if (last && last.chapter_id === l.chapter_id) last.lines.push(l)
    else out.push({ ...l, lines: [l] })
  }
  return out
})
</script>

<style scoped>
.lines-tools {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.lines-search {
  flex: 1 1 14rem;
}

.lines-tools select {
  flex: 0 1 18rem;
  min-width: 0;
}

.lines-chapter + .lines-chapter {
  margin-top: 1.1rem;
}

.lines-chapter h3 {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.2rem 0.7rem;
  margin: 0 0 0.45rem;
  font-size: 0.95rem;
  font-weight: 600;
}

.lines-chapter h3 .faint {
  font-size: 0.78rem;
  font-weight: 400;
}

.lines-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.15rem;
}

.line {
  display: block;
  padding: 0.35rem 0.6rem;
  border-left: 2px solid color-mix(in srgb, var(--accent) 45%, transparent);
  border-radius: 0 8px 8px 0;
  color: var(--text-primary);
  text-decoration: none;
  white-space: pre-line;
}

.line:hover {
  background: var(--glass);
}

.manner {
  margin-right: 0.45rem;
  font-style: italic;
  color: var(--text-muted);
}

.manner::after {
  content: ':';
}

.as {
  margin-left: 0.45rem;
  font-size: 0.78rem;
}

.faint {
  color: var(--text-faint);
}
</style>
