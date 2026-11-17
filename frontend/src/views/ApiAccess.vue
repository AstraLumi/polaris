<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('API access') }}</h1>
        <p class="page-sub">{{ $t('Keys that let a program, such as an LLM chat, read and change your stories through the API. One list for every story.') }}</p>
      </div>
    </header>

    <p v-if="loadError" class="error-banner" role="alert">{{ loadError }} <button type="button" class="retry" @click="load">{{ $t('Try again') }}</button></p>
    <p v-else-if="!info" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <section class="glass-panel card">
        <h2>{{ $t('The API') }}</h2>
        <template v-if="info.enabled">
          <p class="state on">{{ $t('On, at port {port}.', { port: info.port }) }}</p>
          <p class="lede">{{ $t('Only the API answers on that port, and only with a key. Keep the app\'s own port private: anyone who reaches it can make keys.') }}</p>
          <dl class="routes">
            <dt>{{ $t('For MCP clients (Claude, Open WebUI…)') }}</dt><dd><code>/mcp</code></dd>
            <dt>{{ $t('For OpenAPI clients') }}</dt><dd><code>/openapi.json</code></dd>
            <dt>{{ $t('Each story\'s API') }}</dt><dd><code>/stories/&lt;id&gt;/…</code></dd>
          </dl>
          <p class="hint">{{ $t('Send the key as the header Authorization: Bearer <key>. Requests from a browser are refused.') }}</p>
        </template>
        <template v-else>
          <p class="state">{{ $t('Off.') }}</p>
          <p class="lede">{{ $t('To turn it on, set API_PORT in the .env file beside docker-compose.yml and restart Polaris. The README explains how to reach it safely from outside.') }}</p>
        </template>
      </section>

      <section class="glass-panel card">
        <h2>{{ $t('New key') }}</h2>
        <form class="key-form" @submit.prevent="make">
          <label class="field">
            <span>{{ $t('Name') }}</span>
            <input v-model="draft.name" type="text" maxlength="60" :placeholder="$t('e.g. Claude on my laptop')" required />
          </label>
          <fieldset class="field">
            <legend>{{ $t('Access') }}</legend>
            <label class="choice"><input v-model="draft.write" type="radio" :value="false" /> {{ $t('Read only') }}</label>
            <label class="choice"><input v-model="draft.write" type="radio" :value="true" /> {{ $t('Read and change') }}</label>
          </fieldset>
          <fieldset class="field">
            <legend>{{ $t('Stories') }}</legend>
            <label class="choice"><input v-model="draft.all" type="radio" :value="true" /> {{ $t('Every story, new ones too') }}</label>
            <label class="choice"><input v-model="draft.all" type="radio" :value="false" /> {{ $t('Only these:') }}</label>
            <div v-if="!draft.all" class="story-picks">
              <label v-for="s in stories" :key="s.id" class="choice"><input v-model="draft.stories" type="checkbox" :value="s.id" /> {{ s.name }}</label>
            </div>
          </fieldset>
          <label class="field">
            <span>{{ $t('Works until (optional)') }}</span>
            <input v-model="draft.expires" type="date" :min="today" />
          </label>
          <p v-if="formError" class="error-banner" role="alert">{{ formError }}</p>
          <div class="actions">
            <button type="submit" class="btn btn-primary" :disabled="busy || !draft.name.trim() || (!draft.all && !draft.stories.length)">{{ $t('Make the key') }}</button>
          </div>
        </form>

        <div v-if="made" class="made" role="status">
          <p><strong>{{ $t('Copy the key now: it is shown only this once.') }}</strong></p>
          <div class="secret">
            <code>{{ made.key }}</code>
            <button type="button" class="btn btn-ghost" @click="copy(made.key)">{{ copied === made.key ? $t('Copied') : $t('Copy') }}</button>
          </div>
          <p class="hint">{{ $t('If a gateway in front of Polaris lists keys by their SHA-256, this is the key\'s:') }}</p>
          <div class="secret">
            <code>{{ made.info.sha256 }}</code>
            <button type="button" class="btn btn-ghost" @click="copy(made.info.sha256)">{{ copied === made.info.sha256 ? $t('Copied') : $t('Copy') }}</button>
          </div>
          <button type="button" class="btn btn-ghost" @click="made = null">{{ $t('Done') }}</button>
        </div>
      </section>

      <section class="glass-panel card">
        <h2>{{ $t('Keys') }}</h2>
        <p v-if="!info.keys.length" class="note">{{ $t('No keys yet.') }}</p>
        <ul v-else class="keys">
          <li v-for="k in info.keys" :key="k.id" :class="{ expired: isExpired(k) }">
            <div class="key-main">
              <strong>{{ k.name }}</strong>
              <code class="hint-code">{{ k.hint }}…</code>
              <span class="tag">{{ k.write ? $t('Read and change') : $t('Read only') }}</span>
            </div>
            <small class="key-meta">
              {{ storiesText(k) }} ·
              <template v-if="k.expires_at">{{ isExpired(k) ? $t('expired {date}', { date: dayText(k.expires_at) }) : $t('until {date}', { date: dayText(k.expires_at) }) }} · </template>
              {{ k.last_used_at ? $t('last used {when}', { when: timeAgo(k.last_used_at) }) : $t('never used') }}
            </small>
            <button type="button" class="btn btn-ghost danger" @click="revoke(k)">{{ $t('Revoke') }}</button>
          </li>
        </ul>
      </section>

      <section class="glass-panel card">
        <h2>{{ $t('Recent changes through the API') }}</h2>
        <p class="lede">{{ $t('Every change a key made, and every one refused. Before a story\'s first change of the day, Polaris keeps a copy of it (Earlier copies in the story picker).') }}</p>
        <p v-if="!info.log.length" class="note">{{ $t('Nothing yet.') }}</p>
        <table v-else class="log">
          <thead><tr><th>{{ $t('When') }}</th><th>{{ $t('Key') }}</th><th>{{ $t('Story') }}</th><th>{{ $t('Change') }}</th><th>{{ $t('Result') }}</th></tr></thead>
          <tbody>
            <tr v-for="(row, i) in info.log" :key="i">
              <td :title="exactTime(row.at)">{{ timeAgo(row.at) }}</td>
              <td>{{ row.key_name }}</td>
              <td>{{ storyName(row.story_id) }}</td>
              <td><code>{{ row.method }} {{ row.path }}</code></td>
              <td :class="row.status < 300 ? 'ok' : 'bad'">{{ row.status < 300 ? $t('done') : $t('refused ({status})', { status: row.status }) }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>
  </div>
</template>

<script setup>
// Settings → API access (backend/apikeys.go): the keys of the API listener
// (backend/apiserver.go). The keys are global, not a story's.
import { ref, reactive, onMounted } from 'vue'
import { t, tr, tn } from '../i18n'
import { stories, loadStories } from '../stories'
import { askConfirm } from '../confirm'
import { timeAgo, exactTime } from '../timeAgo'

// Messages the server can give (backend/apikeys.go), listed so they get translated.
void [
  tr('a key needs a name (60 characters at most)'), tr('story not found'), tr('the expiry date must be today or later'),
  tr('there are already 50 keys; revoke one first'), tr('key not found'), tr('failed to revoke the key'), tr('invalid request'),
]

const info = ref(null)
const loadError = ref('')
const formError = ref('')
const busy = ref(false)
const made = ref(null)
const copied = ref('')
const today = new Date().toISOString().slice(0, 10)
const draft = reactive({ name: '', write: false, all: true, stories: [], expires: '' })

async function send(url, options, fallback) {
  let res
  try {
    res = await fetch(url, options)
  } catch (e) {
    throw new Error(t("can't reach the server"))
  }
  if (!res.ok) {
    let msg = ''
    try {
      msg = (await res.text()).trim()
    } catch (e) {
      /* no body */
    }
    throw new Error(t(msg || fallback))
  }
  return res.json()
}

async function load() {
  loadError.value = ''
  try {
    info.value = await send('/api/api-access', undefined, 'failed to load')
  } catch (e) {
    loadError.value = e.message
  }
}

onMounted(() => {
  load()
  loadStories().catch(() => {})
})

async function make() {
  formError.value = ''
  busy.value = true
  try {
    const body = { name: draft.name.trim(), write: draft.write, stories: draft.all ? [] : draft.stories, expires: draft.expires || '' }
    made.value = await send('/api/api-access/keys', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }, 'failed to save')
    Object.assign(draft, { name: '', write: false, all: true, stories: [], expires: '' })
    await load()
  } catch (e) {
    formError.value = e.message
  } finally {
    busy.value = false
  }
}

async function revoke(k) {
  const ok = await askConfirm(t('Revoke the key “{name}”?', { name: k.name }), {
    message: t('Whatever uses it stops working at once. This can\'t be undone: a new key would have to be made.'),
    confirmLabel: t('Revoke'),
  })
  if (!ok) return
  try {
    await send(`/api/api-access/keys/${k.id}`, { method: 'DELETE' }, 'failed to revoke the key')
    await load()
  } catch (e) {
    loadError.value = e.message
  }
}

async function copy(text) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = text
  } catch (e) {
    copied.value = ''
  }
}

const storyName = (id) => stories.value.find((s) => s.id === id)?.name || id || '—'
const storiesText = (k) => (k.stories.length ? tn('{n} story', '{n} stories', k.stories.length, { n: k.stories.length }) + ': ' + k.stories.map(storyName).join(', ') : t('every story'))
const isExpired = (k) => !!k.expires_at && k.expires_at < today
const dayText = (d) => new Date(d + 'T12:00:00').toLocaleDateString()
</script>

<style scoped>
.card { padding: 1.25rem 1.5rem; margin-bottom: 1.25rem; }
.card h2 { margin: 0 0 0.6rem; }
.state { font-weight: 600; margin: 0 0 0.5rem; }
.state.on { color: var(--accent); }
.routes { display: grid; grid-template-columns: max-content 1fr; gap: 0.3rem 1rem; margin: 0.75rem 0; }
.routes dt { color: var(--text-muted); }
.routes dd { margin: 0; }
.key-form { display: grid; gap: 0.9rem; max-width: 34rem; }
.field { display: grid; gap: 0.35rem; border: 0; padding: 0; margin: 0; }
.field legend, .field > span { font-weight: 600; margin-bottom: 0.2rem; }
.choice { display: flex; gap: 0.45rem; align-items: center; }
.story-picks { display: grid; gap: 0.25rem; padding-left: 1.5rem; }
.actions { display: flex; gap: 0.5rem; }
.made { margin-top: 1.25rem; padding: 1rem; border-radius: 10px; background: rgba(var(--wash), 0.05); display: grid; gap: 0.6rem; }
.made p { margin: 0; }
.secret { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; }
.secret code { overflow-wrap: anywhere; padding: 0.35rem 0.55rem; border-radius: 6px; background: rgba(var(--wash), 0.08); user-select: all; }
.keys { list-style: none; padding: 0; margin: 0; display: grid; gap: 0.6rem; }
.keys li { display: grid; grid-template-columns: 1fr auto; gap: 0.2rem 1rem; align-items: center; padding: 0.6rem 0; border-bottom: 1px solid var(--line); }
.keys li.expired { opacity: 0.6; }
.key-main { display: flex; gap: 0.6rem; align-items: baseline; flex-wrap: wrap; }
.key-meta { grid-column: 1; color: var(--text-muted); }
.keys li > .btn { grid-row: 1 / span 2; grid-column: 2; }
.hint-code { color: var(--text-muted); }
.tag { font-size: 0.8rem; padding: 0.1rem 0.5rem; border-radius: 999px; background: rgba(var(--wash), 0.08); }
.danger { color: var(--danger-text); }
.log { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
.log th, .log td { text-align: left; padding: 0.35rem 0.5rem; border-bottom: 1px solid var(--line); vertical-align: top; }
.log code { overflow-wrap: anywhere; }
.log .bad { color: var(--danger-text); }
@media (max-width: 640px) {
  .log thead { display: none; }
  .log tr { display: grid; padding: 0.4rem 0; }
  .log td { border: 0; padding: 0.1rem 0; }
}
</style>
