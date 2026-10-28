// The spellchecker on the writing page (docs/EDITOR.md, "Spellchecker"):
// on or off (remembered in this browser), which dictionary (the book's
// language, else the interface's), the story's words (names from the wiki,
// the words added to the story's dictionary, the words ignored until the
// page is left) and the worker that holds the dictionary. spellPlugin.js
// reads it through `forPlugin()`.
import { ref, shallowRef, computed, watch, onBeforeUnmount } from 'vue'
import { locale, t } from '../i18n'
import { spellingApi } from './api.js'
import { createSpellClient } from './spellClient.js'
import { spellLanguage, wordSet, storyWords, cleanWord, DICTIONARIES } from './spellWords.js'

const ON_KEY = 'polaris-write-spell'
const CAPS_KEY = 'polaris-write-spell-caps'
const read = (k, d) => {
  try {
    return localStorage.getItem(k) ?? d
  } catch (e) {
    return d
  }
}
const write = (k, v) => {
  try {
    localStorage.setItem(k, v)
  } catch (e) {
    /* this visit only */
  }
}

// Ignored words last while the app stays open (the story is fixed per page load).
const ignored = ref([])

// getLanguage(): the book's meta.language. names: useStoryNames().
export function useSpellcheck({ getLanguage, names }) {
  const enabled = ref(read(ON_KEY, '1') !== '0')
  const skipCaps = ref(read(CAPS_KEY, '1') !== '0')
  watch(enabled, (v) => write(ON_KEY, v ? '1' : '0'))
  watch(skipCaps, (v) => write(CAPS_KEY, v ? '1' : '0'))

  const language = computed(() => spellLanguage(getLanguage(), locale.value))
  const status = ref('off') // off | loading | ready | none | error
  const added = ref([])
  const error = ref('')
  const gen = ref(1)
  let client = null

  const nameWords = computed(() => storyWords((names.index.value || []).map((e) => e.label)))
  const known = shallowRef(wordSet([]))
  watch([nameWords, added, ignored], () => {
    known.value = wordSet([...nameWords.value, ...added.value, ...ignored.value])
    gen.value++
  }, { immediate: true, deep: true })
  watch(skipCaps, () => gen.value++)

  async function loadWords() {
    try {
      added.value = (await spellingApi.words()).words || []
    } catch (e) {
      /* the dictionary works without them */
    }
  }

  let token = 0
  async function start() {
    const my = ++token
    const code = language.value.code
    if (!enabled.value) return (status.value = 'off')
    if (!code) return (status.value = 'none')
    client ||= createSpellClient()
    status.value = 'loading'
    try {
      await client.use(code)
      if (my !== token) return
      status.value = 'ready'
      gen.value++
    } catch (e) {
      if (my === token) status.value = 'error'
    }
  }
  watch([enabled, () => language.value.code], start)
  start()
  loadWords()
  // Another tab may have added words.
  window.addEventListener('focus', loadWords)

  onBeforeUnmount(() => {
    token++
    window.removeEventListener('focus', loadWords)
    client?.dispose()
  })

  async function change(patch) {
    error.value = ''
    try {
      added.value = (await spellingApi.change(patch)).words || []
    } catch (e) {
      error.value = e.message
    }
  }

  return {
    enabled,
    skipCaps,
    language,
    status,
    added,
    ignored,
    error,
    gen,
    // "English (US)", or the tag that has no dictionary.
    dictionaryName: computed(() => (language.value.code ? DICTIONARIES[language.value.code].name : language.value.tag)),
    statusLabel: computed(() => {
      const name = language.value.code ? DICTIONARIES[language.value.code].name : ''
      return {
        off: t('Spelling: off'),
        loading: t('Spelling: loading {name}…', { name }),
        ready: t('Spelling: {name}', { name }),
        none: t('Spelling: no dictionary for “{lang}”', { lang: language.value.tag }),
        error: t("Spelling: the dictionary didn't load"),
      }[status.value]
    }),
    loadWords,
    retry: start,
    addWord: (w) => {
      const c = cleanWord(w)
      return c ? change({ add: [c] }) : Promise.resolve()
    },
    removeWord: (w) => change({ remove: [w] }),
    ignore: (w) => {
      if (!ignored.value.includes(w)) ignored.value = [...ignored.value, w]
    },
    forgetIgnored: () => (ignored.value = []),
    suggest: (w) => (client && status.value === 'ready' ? client.suggest(w) : Promise.resolve([])),
    // What spellPlugin reads; null while off or not ready.
    forPlugin() {
      if (!enabled.value || status.value !== 'ready' || !client) return null
      const set = known.value
      return { gen: gen.value, skipCaps: skipCaps.value, known: (w) => set.has(w), wrong: (words) => client.wrong(words) }
    },
  }
}
