<template>
  <aside class="side-panel glass-panel panel-solid offline-panel" :aria-label="$t('Offline writing')">
    <header class="side-head">
      <h2>{{ $t('Offline writing') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <p class="state" :class="{ 'is-away': !offline.connected.value }">
      {{ offline.connected.value ? $t('Connected to the server.') : $t('No connection: what you write is kept on this device.') }}
    </p>

    <label class="check-row">
      <input type="checkbox" :checked="offline.available.value" :disabled="!offline.supported || offline.busy.value || (!offline.available.value && !offline.connected.value)" @change="$emit('toggle', $event.target.checked)" />
      {{ $t('Available offline') }}
    </label>
    <p class="hint">{{ $t("Keeps this book's chapters, styles, fonts and pictures on this device, so you can open it and write with no connection. What you write is sent when the server is back.") }}</p>
    <p v-if="!offline.supported" class="hint warn">{{ $t('Offline writing needs Polaris opened over HTTPS (or on this computer, as localhost).') }}</p>

    <p v-if="offline.progress.value" class="hint">
      {{
        offline.progress.value.what === 'files'
          ? $t('Keeping pictures and fonts… {done} of {total}', offline.progress.value)
          : $t('Keeping chapters… {done} of {total}', offline.progress.value)
      }}
    </p>
    <p v-else-if="offline.available.value && offline.record.value?.saved_at" class="hint">
      {{ $t('Kept on this device, brought up to date {when}.', { when: ago(offline.record.value.saved_at) }) }}
      <button v-if="offline.connected.value" type="button" class="link-btn" :disabled="offline.busy.value" @click="offline.refresh()">{{ $t('Update now') }}</button>
    </p>
    <p v-if="offline.error.value" class="error-banner">{{ offline.error.value }}</p>
    <p v-if="offline.available.value && offline.persisted.value === false" class="hint">{{ $t('The browser may clear what it keeps when the device runs low on space. Write with a connection now and then.') }}</p>

    <template v-if="offline.dirty.value.length">
      <h3>{{ $t('Not sent yet') }}</h3>
      <ul class="list">
        <li v-for="c in offline.dirty.value" :key="c.id">
          <button v-if="c.volume === volume" type="button" class="link-btn" @click="$emit('open', c.id)">{{ titleOf(c.id) }}</button>
          <RouterLink v-else :to="`/write/${c.volume}?chapter=${c.id}`">{{ titleOf(c.id) }}</RouterLink>
          <span v-if="c.missing" class="tag">{{ $t('deleted on the server') }}</span>
          <span v-else-if="c.conflict" class="tag">{{ $t('changed elsewhere too: open it to choose') }}</span>
          <span v-else-if="c.edited_at" class="when">{{ ago(c.edited_at) }}</span>
        </li>
      </ul>
      <button v-if="offline.connected.value" type="button" class="btn btn-ghost small" :disabled="offline.busy.value" @click="$emit('sync')">{{ $t('Send now') }}</button>
    </template>

    <template v-if="others.length">
      <h3>{{ $t('Other books kept offline') }}</h3>
      <ul class="list">
        <li v-for="b in others" :key="b.volume">
          <RouterLink :to="`/write/${b.volume}`">{{ b.volume === '0' ? $t('Chapters without a volume') : b.title || $t('Book') }}</RouterLink>
        </li>
      </ul>
    </template>
  </aside>
</template>

<script setup>
// Offline writing (docs/EDITOR.md, "Offline writing"): keep this book on the
// device or forget it, how up to date it is, what hasn't reached the server.
import { computed } from 'vue'
import { t } from '../../i18n'
import { ago } from '../../editor/when.js'

const props = defineProps({
  offline: { type: Object, required: true }, // useOfflineBook()
  chapters: { type: Array, default: () => [] },
  volume: { type: String, required: true },
})
defineEmits(['close', 'toggle', 'open', 'sync'])

const others = computed(() => props.offline.books.value.filter((b) => b.volume !== props.volume))
const titleOf = (id) => props.chapters.find((c) => c.id === id)?.title || t('Chapter {id}', { id })
</script>

<style scoped>
h3 {
  margin: 1rem 0 0.4rem;
  font-size: 0.9rem;
}

.state {
  margin: 0 0 0.6rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.state.is-away {
  color: var(--accent);
  font-weight: 600;
}

.check-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.88rem;
  margin: 0.3rem 0;
}

.check-row input {
  width: auto;
  min-height: 0;
}

.hint {
  margin: 0.3rem 0 0.5rem;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
}

.hint.warn {
  color: var(--accent);
}

.link-btn {
  background: none;
  border: none;
  padding: 0;
  color: var(--accent);
  text-decoration: underline;
  cursor: pointer;
  font: inherit;
}

.list {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: grid;
  gap: 0.25rem;
  font-size: 0.85rem;
}

.list a {
  color: var(--accent);
}

.tag,
.when {
  margin-left: 0.4rem;
  font-size: 0.72rem;
  color: var(--text-faint);
}

.tag {
  color: var(--accent);
}
</style>
