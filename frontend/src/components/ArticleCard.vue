<template>
  <div class="article-card">
    <div class="card-head">
      <span class="pic"><IconImage :src="article.picture" :name="article.name" :alt="article.name" /></span>
      <span class="head-text">
        <strong>{{ article.name }}</strong>
        <small>{{ $t(TYPE_LABELS[article.type]) }}</small>
      </span>
    </div>
    <dl v-if="facts.length" class="facts">
      <template v-for="f in facts" :key="f.key">
        <dt>{{ $t(FACT_LABELS[f.key]) }}</dt>
        <dd>{{ f.text }}</dd>
      </template>
    </dl>
    <p v-if="text" class="text">{{ text }}</p>
    <p v-else class="text is-empty">{{ $t('Nothing written here yet: only the facts from the story') }}</p>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import IconImage from './IconImage.vue'
import { TYPE_LABELS, FACT_LABELS } from '../wiki'
import { articleText } from '../articlePreview'
import { fullDate, dateParts } from '../calendar'
import { t } from '../i18n'

// A small summary of a wiki article: picture, kind, a few facts and the
// start of its text. Shown when hovering a link to it (LinkPreviewLayer).
const props = defineProps({ article: { type: Object, required: true } })

const text = computed(() => articleText(props.article))

const facts = computed(() =>
  (props.article.facts || [])
    .map((f) => {
      let value = f.link ? f.link.name : f.value
      if (f.kind === 'date' && dateParts(f.value)) value = fullDate(f.value)
      if (f.kind === 'word') value = t(f.value)
      return { key: f.key, text: value }
    })
    .filter((f) => f.text && FACT_LABELS[f.key])
    .slice(0, 3),
)
</script>

<style scoped>
.article-card {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.card-head {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.pic {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  overflow: hidden;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  font-weight: 700;
}

.pic :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.pic :deep(svg) {
  width: 65%;
  height: 65%;
}

.head-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.head-text strong {
  font-family: 'Fraunces', serif;
  font-size: 1.02rem;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.head-text small {
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-faint);
}

.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.15rem 0.6rem;
  margin: 0;
  font-size: 0.76rem;
}

.facts dt {
  color: var(--text-faint);
}

.facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.text {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-muted);
}

.text.is-empty {
  font-style: italic;
  color: var(--text-faint);
}
</style>
