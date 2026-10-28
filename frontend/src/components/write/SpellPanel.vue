<template>
  <aside class="side-panel glass-panel panel-solid spell-panel" :aria-label="$t('Spelling')">
    <header class="side-head">
      <h2>{{ $t('Spelling') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <label class="check-row">
      <input v-model="spell.enabled.value" type="checkbox" />
      {{ $t('Check spelling as I write') }}
    </label>
    <p class="hint">{{ $t("While it is on, the browser's own spelling check is off in the editor.") }}</p>

    <label class="field">
      <span>{{ $t("Dictionary (the book's language)") }}</span>
      <select :value="spell.language.value.code || ''" @change="$emit('language', $event.target.value)">
        <option v-if="!spell.language.value.code" value="" disabled>{{ $t('No dictionary for “{lang}”', { lang: spell.language.value.tag }) }}</option>
        <option v-for="(d, code) in DICTIONARIES" :key="code" :value="code">{{ d.name }}</option>
      </select>
    </label>
    <p class="hint">
      {{ spell.statusLabel.value }}<br />
      <template v-if="spell.language.value.from === 'interface'">{{ $t('The book has no language set, so the interface’s is used.') }}</template>
      <button v-if="spell.status.value === 'error'" type="button" class="link-btn" @click="spell.retry()">{{ $t('Try again') }}</button>
    </p>

    <label class="check-row">
      <input v-model="spell.skipCaps.value" type="checkbox" />
      {{ $t('Ignore words in capitals') }}
    </label>
    <p class="hint">{{ $t("The story's names (characters, places, factions, creatures, assets…), nicknames and former names always count as words.") }}</p>

    <h3>{{ $t("This story's dictionary") }}</h3>
    <form class="add-row" @submit.prevent="add">
      <input v-model="typed" type="text" maxlength="64" :placeholder="$t('Add a word')" :aria-label="$t('Add a word')" />
      <button type="submit" class="btn btn-ghost small" :disabled="!typed.trim()">{{ $t('Add') }}</button>
    </form>
    <p v-if="spell.error.value" class="error-banner">{{ spell.error.value }}</p>
    <p v-if="!spell.added.value.length" class="hint">{{ $t('Words you add with “Add to dictionary” show here.') }}</p>
    <ul v-else class="word-list">
      <li v-for="w in sorted" :key="w">
        <span>{{ w }}</span>
        <button type="button" class="btn btn-ghost small" :aria-label="$t('Remove {word}', { word: w })" :title="$t('Remove {word}', { word: w })" @click="spell.removeWord(w)">✕</button>
      </li>
    </ul>

    <template v-if="spell.ignored.value.length">
      <h3>{{ $t('Ignored for now') }}</h3>
      <p class="hint">{{ spell.ignored.value.join(', ') }}</p>
      <button type="button" class="btn btn-ghost small" @click="spell.forgetIgnored()">{{ $t('Check them again') }}</button>
    </template>
  </aside>
</template>

<script setup>
// The spellchecker's options (docs/EDITOR.md, "Spellchecker"): on or off,
// the dictionary (it sets the book's language), words in capitals, and the
// story's own dictionary (add, see, remove).
import { ref, computed } from 'vue'
import { locale } from '../../i18n'
import { DICTIONARIES } from '../../editor/spellWords.js'

const props = defineProps({ spell: { type: Object, required: true } }) // useSpellcheck()
defineEmits(['close', 'language'])

const typed = ref('')
const sorted = computed(() => [...props.spell.added.value].sort((a, b) => a.localeCompare(b, locale.value)))
async function add() {
  await props.spell.addWord(typed.value)
  if (!props.spell.error.value) typed.value = ''
}
</script>

<style scoped>
h3 {
  margin: 1rem 0 0.4rem;
  font-size: 0.9rem;
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

.field {
  display: grid;
  gap: 0.25rem;
  margin-top: 0.6rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.hint {
  margin: 0.3rem 0 0.5rem;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
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

.add-row {
  display: flex;
  gap: 0.4rem;
}

.add-row input {
  flex: 1;
  min-width: 0;
}

.word-list {
  list-style: none;
  margin: 0.5rem 0 0;
  padding: 0;
  display: grid;
  gap: 0.1rem;
  max-height: 18rem;
  overflow-y: auto;
}

.word-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem;
  padding: 0.1rem 0.2rem 0.1rem 0.5rem;
  border-radius: 7px;
  font-size: 0.86rem;
}

.word-list li:hover {
  background: var(--glass);
}
</style>
