<template>
  <aside ref="root" class="side-panel glass-panel panel-solid comments-panel" :aria-label="$t('Comments')">
    <header class="side-head">
      <h2>{{ $t('Comments') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <div class="cm-tools">
      <button type="button" class="btn btn-primary small" :disabled="readOnly" :title="$t('Comment on the selected text') + ' (Ctrl+Alt+M)'" @click="$emit('new')">{{ $t('New comment') }}</button>
      <label class="cm-check">
        <input type="checkbox" :checked="showResolved" @change="$emit('show-resolved', $event.target.checked)" />
        {{ $t('Show resolved') }}<span v-if="resolvedCount"> ({{ resolvedCount }})</span>
      </label>
    </div>

    <p v-if="error" class="error-banner">{{ error }}</p>
    <p v-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>
    <p v-else-if="!threads.length" class="hint">{{ $t('No comments here. Select some text and press Ctrl+Alt+M to comment on it.') }}</p>

    <ol class="thread-list">
      <li
        v-for="th in threads"
        :key="th.comment.id"
        :ref="(el) => setItem(th.comment.id, el)"
        class="thread"
        :class="{ 'is-active': th.comment.id === activeId, 'is-resolved': th.comment.resolved }"
        @click="onThreadClick(th, $event)"
      >
        <p class="quote" :title="$t('Show this text')">“{{ th.quote }}”</p>

        <div class="entry">
          <form v-if="editing === th.comment.id" class="cm-form" @submit.prevent="saveEdit(th.comment)" @click.stop>
            <textarea
              :ref="(el) => el && editing === th.comment.id && focusOnce(el)"
              v-model="draft"
              rows="3"
              :maxlength="COMMENT_MAX"
              :aria-label="$t('Comment')"
              :placeholder="$t('Your note…')"
              @keydown.enter.ctrl.prevent="saveEdit(th.comment)"
              @keydown.enter.meta.prevent="saveEdit(th.comment)"
              @keydown.esc.stop.prevent="cancelEdit(th.comment)"
            ></textarea>
            <div class="cm-actions">
              <button type="submit" class="btn btn-primary small" :disabled="busy">{{ $t('Save') }}</button>
              <button type="button" class="btn btn-ghost small" :disabled="busy" @click="cancelEdit(th.comment)">{{ $t('Cancel') }}</button>
            </div>
          </form>
          <template v-else>
            <p class="cm-text" :class="{ 'is-empty': !th.comment.text }">{{ th.comment.text || $t('(empty)') }}</p>
            <div class="cm-meta">
              <span :title="exact(th.comment.created_at)">{{ ago(th.comment.created_at) }}</span>
              <span v-if="th.comment.resolved" class="badge">{{ $t('Resolved') }}</span>
              <span class="spacer"></span>
              <button v-if="th.comment.resolved" type="button" class="btn btn-ghost small" :disabled="busy" @click.stop="$emit('resolve', th.comment.id, false)">{{ $t('Reopen') }}</button>
              <button v-else type="button" class="btn btn-ghost small" :disabled="busy" :title="$t('Mark as resolved: its highlight goes')" @click.stop="$emit('resolve', th.comment.id, true)">{{ $t('Resolve') }}</button>
              <RowMenu :label="$t('Comment actions')" :items="menu(th.comment)" @click.stop />
            </div>
          </template>
        </div>

        <ul v-if="th.replies.length" class="replies">
          <li v-for="r in th.replies" :key="r.id" class="entry reply">
            <form v-if="editing === r.id" class="cm-form" @submit.prevent="saveEdit(r)" @click.stop>
              <textarea
                :ref="(el) => el && editing === r.id && focusOnce(el)"
                v-model="draft"
                rows="2"
                :maxlength="COMMENT_MAX"
                :aria-label="$t('Reply')"
                @keydown.enter.ctrl.prevent="saveEdit(r)"
                @keydown.enter.meta.prevent="saveEdit(r)"
                @keydown.esc.stop.prevent="cancelEdit(r)"
              ></textarea>
              <div class="cm-actions">
                <button type="submit" class="btn btn-primary small" :disabled="busy || !draft.trim()">{{ $t('Save') }}</button>
                <button type="button" class="btn btn-ghost small" :disabled="busy" @click="cancelEdit(r)">{{ $t('Cancel') }}</button>
              </div>
            </form>
            <template v-else>
              <p class="cm-text">{{ r.text }}</p>
              <div class="cm-meta">
                <span :title="exact(r.created_at)">{{ ago(r.created_at) }}</span>
                <span class="spacer"></span>
                <RowMenu :label="$t('Reply actions')" :items="menu(r)" @click.stop />
              </div>
            </template>
          </li>
        </ul>

        <form v-if="replying === th.comment.id" class="cm-form reply-form" @submit.prevent="sendReply(th.comment)" @click.stop>
          <textarea
            :ref="(el) => el && focusOnce(el)"
            v-model="replyDraft"
            rows="2"
            :maxlength="COMMENT_MAX"
            :aria-label="$t('Reply')"
            :placeholder="$t('Reply…')"
            @keydown.enter.ctrl.prevent="sendReply(th.comment)"
            @keydown.enter.meta.prevent="sendReply(th.comment)"
            @keydown.esc.stop.prevent="replying = null"
          ></textarea>
          <div class="cm-actions">
            <button type="submit" class="btn btn-primary small" :disabled="busy || !replyDraft.trim()">{{ $t('Reply') }}</button>
            <button type="button" class="btn btn-ghost small" :disabled="busy" @click="replying = null">{{ $t('Cancel') }}</button>
          </div>
        </form>
        <button v-else-if="!th.comment.resolved && editing !== th.comment.id" type="button" class="link-btn reply-btn" @click.stop="openReply(th.comment)">{{ $t('Reply') }}</button>
      </li>
    </ol>
    <p v-if="detached" class="hint">{{ $tn('{n} comment is on text that was deleted; undo or restoring a version brings it back.', '{n} comments are on text that was deleted; undo or restoring a version brings them back.', detached) }}</p>
  </aside>
</template>

<script setup>
// The chapter's comments beside the page (stage 5; docs/EDITOR.md,
// "Comments"): threads in the order of the text with what they cover, edit,
// reply, resolve or reopen, delete. The writing page holds the comments and
// does the calls; this panel shows them and says what the writer did.
import { ref, watch, nextTick } from 'vue'
import { t } from '../../i18n'
import { ago, exact } from '../../editor/when.js'
import { COMMENT_MAX } from '../../editor/comments.js'
import RowMenu from '../RowMenu.vue'

const props = defineProps({
  threads: { type: Array, required: true }, // panelThreads(…).threads
  detached: { type: Number, default: 0 },
  resolvedCount: { type: Number, default: 0 },
  showResolved: { type: Boolean, default: false },
  activeId: { type: Number, default: null },
  editId: { type: Number, default: null }, // a comment to edit now (a new one)
  loading: { type: Boolean, default: false },
  busy: { type: Boolean, default: false },
  readOnly: { type: Boolean, default: false },
  error: { type: String, default: '' },
})
const emit = defineEmits(['close', 'new', 'select', 'save', 'cancel-new', 'reply', 'resolve', 'remove', 'show-resolved'])

const root = ref(null)
const editing = ref(null)
const draft = ref('')
const replying = ref(null)
const replyDraft = ref('')
const items = new Map()
const setItem = (id, el) => (el ? items.set(id, el) : items.delete(id))

// A textarea takes the focus once, when it appears.
const focused = new WeakSet()
function focusOnce(el) {
  if (focused.has(el)) return
  focused.add(el)
  nextTick(() => el.focus())
}

function find(id) {
  for (const th of props.threads) {
    if (th.comment.id === id) return th.comment
    const r = th.replies.find((x) => x.id === id)
    if (r) return r
  }
  return null
}

function startEdit(c) {
  replying.value = null
  editing.value = c.id
  draft.value = c.text || ''
}

watch(
  () => props.editId,
  (id) => {
    const c = id != null && find(id)
    if (c) startEdit(c)
  },
)

// The active comment comes into sight in the list.
watch(
  () => props.activeId,
  (id) => nextTick(() => items.get(id)?.scrollIntoView?.({ block: 'nearest' })),
)

function saveEdit(c) {
  const text = draft.value
  if (c.parent_id && !text.trim()) return
  editing.value = null
  if (text !== c.text) emit('save', c.id, text)
  else if (!text.trim() && !c.parent_id) emit('cancel-new', c.id)
}

// Cancelling a comment never written (a new, empty one) takes it away.
function cancelEdit(c) {
  editing.value = null
  if (!c.parent_id && !c.text) emit('cancel-new', c.id)
}

function openReply(c) {
  editing.value = null
  replying.value = c.id
  replyDraft.value = ''
}

function sendReply(c) {
  const text = replyDraft.value
  if (!text.trim()) return
  replying.value = null
  emit('reply', c.id, text)
}

function onThreadClick(th, e) {
  if (e.target.closest('button, a, textarea, input, form')) return
  emit('select', th.comment.id)
}

function menu(c) {
  const isReply = !!c.parent_id
  return [
    { label: t('Edit'), action: () => startEdit(c) },
    !isReply && { label: c.resolved ? t('Reopen') : t('Resolve'), action: () => emit('resolve', c.id, !c.resolved) },
    {
      label: isReply ? t('Delete reply') : t('Delete comment'),
      danger: true,
      action: () => {
        if (isReply || !c.text || window.confirm(t('Delete this comment and its replies?'))) emit('remove', c.id)
      },
    },
  ]
}

defineExpose({ root })
</script>

<style scoped>
.cm-tools {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem;
  margin-bottom: 0.6rem;
}

.cm-check {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.hint {
  margin: 0.4rem 0 0.6rem;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
}

.thread-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.45rem;
}

.thread {
  padding: 0.5rem 0.6rem 0.45rem;
  border-radius: 10px;
  border: 1px solid var(--glass-border);
  border-left: 3px solid color-mix(in srgb, var(--accent) 45%, transparent);
  background: color-mix(in srgb, var(--surface) 70%, transparent);
  cursor: pointer;
}

.thread:hover {
  border-color: color-mix(in srgb, var(--accent) 40%, var(--glass-border));
}

.thread.is-active {
  border-color: var(--accent);
  border-left-color: var(--accent);
  background: var(--accent-soft);
}

.thread.is-resolved {
  opacity: 0.65;
  border-left-color: var(--glass-border);
}

.quote {
  margin: 0 0 0.3rem;
  font-family: 'Fraunces', serif;
  font-style: italic;
  font-size: 0.8rem;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cm-text {
  margin: 0;
  font-size: 0.88rem;
  color: var(--text-primary);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.cm-text.is-empty {
  color: var(--text-faint);
  font-style: italic;
}

.cm-meta {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  margin-top: 0.2rem;
  font-size: 0.72rem;
  color: var(--text-faint);
}

.cm-meta .btn {
  min-height: 1.6rem;
  padding: 0 0.5rem;
  font-size: 0.74rem;
}

.cm-meta :deep(.menu-btn) {
  min-height: 1.6rem;
  padding: 0 0.45rem;
}

.spacer {
  flex: 1;
}

.badge {
  padding: 0 0.4rem;
  border-radius: 999px;
  background: var(--glass);
  color: var(--text-muted);
}

.replies {
  list-style: none;
  margin: 0.35rem 0 0;
  padding: 0 0 0 0.6rem;
  border-left: 1px solid var(--glass-border);
  display: grid;
  gap: 0.3rem;
}

.cm-form {
  display: grid;
  gap: 0.35rem;
  cursor: default;
}

.cm-form textarea {
  width: 100%;
  min-height: 3.2rem;
  resize: vertical;
  font: inherit;
  font-size: 0.86rem;
}

.cm-actions {
  display: flex;
  gap: 0.35rem;
}

.reply-form {
  margin-top: 0.4rem;
}

.link-btn {
  margin-top: 0.15rem;
  padding: 0;
  background: none;
  border: none;
  color: var(--accent);
  font: inherit;
  font-size: 0.76rem;
  cursor: pointer;
}
</style>
