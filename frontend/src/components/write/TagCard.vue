<template>
  <!-- The hover card of a tagged text in the writing page: who said it
       (picture, name and a few facts, the wiki's character card) and/or which
       spell it casts (cost, class, level). In <body>, never takes the focus
       or the mouse. -->
  <Teleport to="body">
    <Transition name="tag-card">
      <div v-if="card" ref="box" class="tag-card glass-panel panel-solid" :style="{ left: pos.x + 'px', top: pos.y + 'px' }" role="tooltip">
        <section v-if="card.who" class="tag-part">
          <ArticleCard v-if="card.who.article" :article="card.who.article" />
          <p v-else class="tag-name">
            <small>{{ $t('Said by') }}</small>
            <strong>{{ card.who.name }}</strong>
          </p>
        </section>
        <section v-if="card.spell" class="tag-part">
          <div class="spell-head">
            <span v-if="card.spell.icon" class="pic"><IconImage :src="card.spell.icon" :name="card.spell.words.name" :alt="card.spell.words.name" /></span>
            <span class="head-text">
              <strong>{{ card.spell.words.name }}</strong>
              <small>{{ $t('Spell') }}</small>
            </span>
          </div>
          <ul v-if="card.spell.known" class="spell-lines">
            <li>{{ card.spell.words.cost }}</li>
            <li v-if="card.spell.words.source">{{ card.spell.words.source }}</li>
            <li v-if="card.spell.words.level">{{ card.spell.words.level }}</li>
          </ul>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, reactive, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import ArticleCard from '../ArticleCard.vue'
import IconImage from '../IconImage.vue'
import { loadArticle } from '../../articlePreview'
import { spellCard } from '../../editor/spellTag.js'
import { t } from '../../i18n'

// Hovering a tag ("Tag character" underline or "Tag spell" underline) shows
// the card after a short delay. Off while the page's "Show tags" is off.
const props = defineProps({
  enabled: { type: Boolean, default: true },
  loadSpells: { type: Function, default: async () => [] }, // -> the story's spells (cached by the page)
})

const DELAY = 350
const GAP = 8
const card = ref(null) // { who: { name, article } | null, spell: { words, icon, known } | null }
const box = ref(null)
const pos = reactive({ x: -9999, y: -9999 })
let target = null
let timer = 0
let seq = 0

function hide() {
  clearTimeout(timer)
  seq++
  target = null
  card.value = null
}

const tagsOf = (el) => ({
  said: el.closest?.('.write-doc .pm-said') || null,
  spell: el.closest?.('.write-doc .pm-spell') || null,
})

async function show(el) {
  const mine = ++seq
  const { said, spell } = tagsOf(el)
  const out = { who: null, spell: null }
  if (said) {
    const id = +said.dataset.said
    const name = said.dataset.saidName || '?'
    let article = null
    if (id > 0) {
      try {
        article = await loadArticle('character', id)
      } catch (e) {
        article = null // deleted, or the server is out of reach: the name alone
      }
    }
    out.who = { name: article?.name || name, article }
  }
  if (spell) {
    const id = +spell.dataset.spell
    const name = spell.dataset.spellName || '?'
    let found = null
    try {
      found = (await props.loadSpells()).find((s) => s.id === id) || null
    } catch (e) {
      found = null
    }
    out.spell = { known: !!found, icon: found?.icon_path || '', words: found ? spellCard(found, t) : { name, cost: '', source: '', level: '' } }
  }
  if (mine !== seq || target !== el || !el.isConnected || !props.enabled) return
  card.value = out
  await nextTick()
  place(el)
}

function place(el) {
  if (!box.value) return
  const r = el.getBoundingClientRect()
  const w = box.value.offsetWidth
  const h = box.value.offsetHeight
  const vw = document.documentElement.clientWidth
  const vh = document.documentElement.clientHeight
  let y = r.bottom + GAP
  if (y + h > vh - GAP && r.top - h - GAP > GAP) y = r.top - h - GAP
  pos.x = Math.round(Math.min(Math.max(GAP, r.left), vw - w - GAP))
  pos.y = Math.round(Math.max(GAP, y))
}

function onOver(e) {
  if (e.pointerType === 'touch' || !props.enabled) return
  const { said, spell } = tagsOf(e.target)
  const el = spell || said
  if (!el) return
  if (el === target) return
  hide()
  target = el
  timer = setTimeout(() => show(el), DELAY)
}
function onOut(e) {
  if (!target) return
  if (e.relatedTarget && target.contains(e.relatedTarget)) return
  hide()
}

const listeners = [
  ['pointerover', onOver],
  ['pointerout', onOut],
  ['pointerdown', hide],
  ['keydown', hide],
  ['scroll', hide],
  ['wheel', hide],
]
onMounted(() => {
  for (const [ev, fn] of listeners) document.addEventListener(ev, fn, { capture: true, passive: true })
  window.addEventListener('blur', hide)
})
onBeforeUnmount(() => {
  for (const [ev, fn] of listeners) document.removeEventListener(ev, fn, { capture: true })
  window.removeEventListener('blur', hide)
  clearTimeout(timer)
})
watch(
  () => props.enabled,
  (on) => on || hide(),
)
</script>

<style scoped>
.tag-card {
  position: fixed;
  z-index: 990;
  width: min(18rem, calc(100vw - 16px));
  padding: 0.7rem 0.85rem;
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.4);
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
.tag-part + .tag-part {
  border-top: 1px solid var(--border, rgba(128, 128, 128, 0.3));
  padding-top: 0.6rem;
}
.tag-name,
.spell-head {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 0.6rem;
}
.tag-name {
  flex-direction: column;
  align-items: flex-start;
  gap: 0.1rem;
}
.head-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.head-text small,
.tag-name small {
  color: var(--text-muted, inherit);
  opacity: 0.75;
}
.pic {
  width: 2.4rem;
  height: 2.4rem;
  flex: none;
  border-radius: 8px;
  overflow: hidden;
}
.spell-lines {
  list-style: none;
  margin: 0.45rem 0 0;
  padding: 0;
  font-size: 0.85rem;
  display: grid;
  gap: 0.15rem;
}
.tag-card-enter-active,
.tag-card-leave-active {
  transition: opacity 0.14s ease;
}
.tag-card-enter-from,
.tag-card-leave-to {
  opacity: 0;
}
@media (prefers-reduced-motion: reduce) {
  .tag-card-enter-active,
  .tag-card-leave-active {
    transition: none;
  }
}
</style>
