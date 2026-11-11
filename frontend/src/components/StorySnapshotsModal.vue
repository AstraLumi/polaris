<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel panel-solid" role="dialog" aria-modal="true" :aria-label="$t('Earlier copies of {name}', { name: story.name })">
      <h2>{{ $t('Earlier copies of {name}', { name: story.name }) }}</h2>
      <p class="message">
        {{ $t('Polaris keeps a copy of each story on the days it changes: the last 7, then one a week for 4 weeks. A copy opens as a new story beside this one; nothing is overwritten.') }}
      </p>
      <p class="message">
        {{ $t('They hold the text and data, not the pictures (a copy shows the pictures the story has now), and they live on the same disk as the story. Export is still the backup to keep elsewhere.') }}
      </p>

      <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
      <p v-if="copies === null && !error" class="loading-hint">{{ $t('Loading…') }}</p>
      <p v-else-if="copies && !copies.length" class="empty">{{ $t('No copies yet. The first is made within a few minutes of Polaris starting, once the story has been opened.') }}</p>
      <ul v-else-if="copies" class="copies">
        <li v-for="c in copies" :key="c.name">
          <span class="when">
            <strong>{{ exactTime(c.taken_at) }}</strong>
            <small>{{ timeAgo(c.taken_at) }} · {{ size(c.size) }}</small>
          </span>
          <button type="button" class="btn btn-ghost small" :disabled="!!busy" @click="open(c)">
            {{ busy === c.name ? $t('Opening…') : $t('Open as a new story') }}
          </button>
        </li>
      </ul>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Close') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
// The server's own daily copies of a story (backend/snapshots.go), and
// bringing one back as a new story.
import { ref, onMounted } from 'vue'
import { fetchSnapshots, restoreSnapshot } from '../stories'
import { timeAgo, exactTime } from '../timeAgo'

const props = defineProps({ story: { type: Object, required: true } })
const emit = defineEmits(['close', 'restored'])

const copies = ref(null)
const error = ref('')
const busy = ref('')

const size = (bytes) => (bytes >= 1048576 ? (bytes / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(bytes / 1024)) + ' kB')

onMounted(async () => {
  try {
    copies.value = await fetchSnapshots(props.story.id)
  } catch (e) {
    error.value = e.message
  }
})

async function open(c) {
  busy.value = c.name
  error.value = ''
  try {
    emit('restored', await restoreSnapshot(props.story.id, c.name))
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = ''
  }
}
</script>

<style scoped>
.message {
  margin: 0 0 0.6rem;
  font-size: 0.86rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.copies {
  list-style: none;
  margin: 0.75rem 0 0;
  padding: 0;
  max-height: 45vh;
  overflow-y: auto;
}

.copies li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0;
  border-top: 1px solid var(--glass-border);
}

.when {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.when small {
  font-size: 0.76rem;
  color: var(--text-faint);
}
</style>
