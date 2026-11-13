<template>
  <div>
    <p class="lede">
      {{ $t('Keep this story on this device to read all of it with no connection: every page, with its pictures. Nothing can be changed offline except the text of books kept for writing.') }}
    </p>
    <p v-if="!supported" class="hint">{{ $t('This needs HTTPS (or localhost) and a browser that can keep data.') }}</p>
    <template v-else>
      <p v-if="copying?.story === story" class="state" role="status">
        {{ $t('Copying for reading offline… {done} of {total}', { done: copying.done, total: copying.total }) }}
      </p>
      <p v-else-if="info" class="state">
        <template v-if="info.at">
          {{ $t('Kept for reading offline, {when}', { when }) }}
          <template v-if="info.missing"> · {{ $tn('{n} item could not be copied', '{n} items could not be copied', info.missing) }}</template>
        </template>
        <template v-else>{{ $t("The copy for reading offline isn't finished. Update it with the connection on.") }}</template>
      </p>
      <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
      <div class="actions">
        <button type="button" class="btn btn-primary" :disabled="busy || !connected" @click="keep">
          {{ info ? $t('Update the offline copy') : $t('Keep for reading offline') }}
        </button>
        <button v-if="info" type="button" class="btn btn-ghost" :disabled="busy" @click="forgetKept(story)">{{ $t('Remove the offline copy') }}</button>
      </div>
    </template>
  </div>
</template>

<script setup>
// The open story's copy for reading offline (docs/DEVELOPMENT.md, "Reading
// offline"), in Settings; the story picker's "…" menu does the same for any
// story.
import { ref, computed } from 'vue'
import { activeStoryId, currentStory } from '../stories'
import { keptStories, forgetKept } from '../offline/kept.js'
import { offlineSupported } from '../offline/caches.js'
import { connected } from '../offline/connection.js'
import { timeAgo } from '../timeAgo'
import { t } from '../i18n'

const story = activeStoryId
const supported = offlineSupported()
const info = computed(() => keptStories.value[story] || null)
const when = computed(() => (info.value?.at ? timeAgo(new Date(info.value.at).toISOString().slice(0, 19).replace('T', ' ')) : ''))

// keepStory.js's own state once it is loaded (a copy renewing by itself
// shows here too).
const maker = ref(null)
const copying = computed(() => maker.value?.copying.value || null)
const busy = computed(() => !!copying.value)
const error = ref('')
import('../offline/keepStory.js').then((m) => (maker.value = m)).catch(() => {})

async function keep() {
  error.value = ''
  try {
    const m = maker.value || (maker.value = await import('../offline/keepStory.js'))
    await m.keepStory(story, currentStory.value?.name || '')
  } catch (err) {
    error.value = err.message || t("can't reach the server")
  }
}
</script>

<style scoped>
.lede,
.hint {
  margin: 0 0 1.25rem;
  color: var(--text-muted);
  font-size: 0.9rem;
  line-height: 1.55;
}

.state {
  margin: 0 0 0.8rem;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}
</style>
