<template>
  <div class="page">
    <Breadcrumbs :missing="missing" :crumbs="[{ label: $t('Characters'), to: '/characters' }, { label: data?.name }]">
      <ItemLinks v-if="data" type="character" :id="data.character_id" here="sheet" />
    </Breadcrumbs>

    <NotFoundPanel v-if="missing" :message="$t('This character was deleted, or never existed.')">
      <RouterLink to="/characters" class="btn btn-ghost">{{ $t('All characters') }}</RouterLink>
    </NotFoundPanel>
    <p v-else-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else-if="data">
      <header class="identity glass-panel">
        <div class="portrait">
          <img v-if="data.picture_path" :src="data.picture_path" :alt="data.name" class="zoomable" @click="viewPicture(data.picture_path, data.name)" />
          <span v-else class="portrait-fallback">{{ initials }}</span>
        </div>
        <div class="identity-text">
          <h1>
            {{ data.name }} <span class="level">{{ $t('Lv.') }} {{ data.level }}</span>
            <span v-if="data.story.status && data.story.status !== 'alive'" class="status-badge" :class="'is-' + data.story.status">
              {{ statusLabel(data.story.status) }}
            </span>
          </h1>
          <p v-if="data.nickname" class="nickname">"{{ data.nickname }}"</p>
          <p class="class-line">
            <span v-if="data.class_icon" class="class-icon"><IconImage :src="data.class_icon" :name="data.class_name" /></span>
            {{ classLine }}
          </p>
          <p v-if="data.tags && data.tags.length" class="tag-line">
            <RouterLink v-for="tag in data.tags" :key="tag" :to="{ path: '/characters', query: { tag } }" class="char-tag" :title="$t('Everyone tagged #{tag}', { tag })">#{{ tag }}</RouterLink>
          </p>
        </div>
        <div class="identity-actions">
          <RouterLink :to="`/characters/${data.character_id}/versions/${data.id}/edit`" class="btn btn-ghost" :title="$t('Edit the current version')">
            {{ $t('Edit') }}
          </RouterLink>
          <RouterLink :to="`/characters/${data.character_id}/versions`" class="btn btn-ghost" :title="$t('Every version of this character, and what they all share')">
            {{ $t('Versions') }}
          </RouterLink>
          <button type="button" class="btn btn-ghost" :title="$t('Print the whole sheet, or save it as a PDF')" @click="print">
            {{ $t('Print') }}
          </button>
        </div>
      </header>

      <div class="tab-bar">
        <button class="tab-button" :class="{ 'is-active': tab === 'story' }" :title="$t('Who they are: details, description and biography')" @click="tab = 'story'">
          {{ $t('Story') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'build' }" :title="$t('Level, stats and where their bonuses come from')" @click="tab = 'build'">
          {{ $t('Build') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'gear' }" :title="$t('What they have equipped')" @click="tab = 'gear'">
          {{ $t('Gear') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'spells' }" :title="$t('The spells they know')" @click="tab = 'spells'">
          {{ $t('Spells') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'relations' }" :title="$t('Family, friends, rivals and factions')" @click="tab = 'relations'">
          {{ $t('Relations') }}
        </button>
      </div>

      <section v-show="tab === 'story'" class="story-tab glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Story') }}</h2>
        <!-- In groups, as on the wiki's infobox; fields with nothing in them
             wait behind "Show empty fields". -->
        <template v-for="g in storyGroups" :key="g.label">
          <template v-if="g.fields.some(shows)">
            <h3 class="group-title">{{ g.label }}</h3>
            <div class="field-grid">
              <ReadField v-for="f in g.fields.filter(shows)" :key="f.label" :label="f.label" :value="f.value" />
            </div>
          </template>
        </template>
        <div class="long-fields">
          <ReadField v-for="f in longFields.filter(shows)" :key="f.label" :label="f.label" :value="f.value" long />
        </div>
        <p v-if="!storyGroups.some((g) => g.fields.some(shows)) && !longFields.some(shows)" class="nothing-yet">
          {{ $t('Nothing written about them yet.') }}
          <RouterLink :to="`/characters/${data.character_id}/versions/${data.id}/edit`">{{ $t('Edit the current version') }}</RouterLink>
        </p>
        <button v-if="emptyCount" type="button" class="btn btn-ghost small empty-toggle no-print" @click="showEmpty = !showEmpty">
          {{ showEmpty ? $t('Hide empty fields') : $tn('Show {n} empty field', 'Show {n} empty fields', emptyCount) }}
        </button>
      </section>

      <section v-show="tab === 'build'" class="build-tab glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Build') }}</h2>
        <BuildStatsPanel
          :model-value="data.build"
          :computed="data.computed"
          :special-bases="data.special_bases"
          :points="data.points || 0"
          :editable="false"
        />
      </section>

      <section v-show="tab === 'gear'" class="gear-tab-wrap glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Gear') }}</h2>
        <GearTab :model-value="data.gear || {}" :carry-limit="data.computed?.carry_limit || 0" />
      </section>

      <section v-show="tab === 'spells'" class="gear-tab-wrap glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Spells') }}</h2>
        <SpellsTab :model-value="data.spells || []" />
      </section>

      <section v-show="tab === 'relations'" class="gear-tab-wrap glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Relations') }}</h2>
        <RelationsTab :character-id="data.character_id" :character-name="data.name" :point="{ chapterId: data.chapter_id, date: data.version_date }" />
      </section>

      <Pager :prev="neighbours.prev" :next="neighbours.next" :prev-label="$t('← Previous character')" :next-label="$t('Next character →')" :label="$t('Characters')" />
    </template>
  </div>
</template>

<script setup>
import IconImage from '../components/IconImage.vue'
import { fullDate } from '../calendar'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import ItemLinks from '../components/ItemLinks.vue'
import NotFoundPanel from '../components/NotFoundPanel.vue'
import Pager from '../components/Pager.vue'
import { statusLabel } from '../characterStatus'
import { genderLabel } from '../gender'
import { pageTitle, viewPicture } from '../navigation'
import { ref, computed, watch, h } from 'vue'
import { t } from '../i18n'
import { useLinkNames, linkText } from '../linkNames'
import { fetchCurrentVersion, fetchCharacterOptions, isMissing } from '../api'
import BuildStatsPanel from '../components/BuildStatsPanel.vue'
import GearTab from '../components/GearTab.vue'
import SpellsTab from '../components/SpellsTab.vue'
import RelationsTab from '../components/RelationsTab.vue'

const props = defineProps({ id: { type: String, required: true } })

const data = ref(null)
const loading = ref(true)
const loadError = ref('')
const missing = ref(false)
const tab = ref('story')

// Everyone, by name (as the Characters page lists them), for the
// previous/next links at the foot of the sheet.
const everyone = ref([])
fetchCharacterOptions()
  .then((list) => (everyone.value = list))
  .catch(() => {})
const neighbours = computed(() => {
  const i = everyone.value.findIndex((c) => String(c.id) === String(props.id))
  if (i < 0) return { prev: null, next: null }
  const link = (c) => c && { to: `/characters/${c.id}`, name: c.name }
  return { prev: link(everyone.value[i - 1]), next: link(everyone.value[i + 1]) }
})

// Tiny inline component for a label + read-only value pair, so the Story
// tab's markup above stays readable instead of repeating this block 14 times.
// The Story tab's fields, grouped. A value of null, '' (or no deaths) is
// empty; empty fields show only when asked.
const showEmpty = ref(false)
const isEmpty = (v) => v === null || v === undefined || v === ''
const shows = (f) => showEmpty.value || !isEmpty(f.value)
const storyGroups = computed(() => {
  const s = data.value.story
  const f = (label, value) => ({ label, value })
  return [
    {
      label: t('Personal'),
      fields: [
        f(t('Gender'), genderLabel(s.gender)), f(t('Race'), s.race_name), f(t('Age'), s.age), f(t('Body type'), s.body_type_name),
        f(t('Height (cm)'), s.height), f(t('Weight (kg)'), s.weight), f(t('Blood type'), s.blood_type),
        f(t('Status'), statusLabel(s.status)), f(t('Deaths'), s.deaths || null),
      ],
    },
    {
      label: t('Birth'),
      fields: [f(t('Birth date (in-story)'), s.birth_date ? fullDate(s.birth_date) : ''), f(t('Born in'), s.born_in_name || s.born_in), f(t('Nation'), s.nation_name || s.nation)],
    },
    { label: t('On Earth'), fields: [f(t('Earth Name'), s.earth_name), f(t('Earth Birthdate'), s.human_birth_date)] },
  ]
})
// Their [[links]] read as names (linkNames.js).
useLinkNames()
const longFields = computed(() => {
  const s = data.value.story
  return [
    { label: t('Description'), value: linkText(s.description) },
    { label: t('Bio'), value: linkText(s.bio) },
    { label: t('Speech mannerisms'), value: linkText(s.speech_mannerisms) },
  ]
})
const emptyCount = computed(() => [...storyGroups.value.flatMap((g) => g.fields), ...longFields.value].filter((x) => isEmpty(x.value)).length)

const ReadField = {
  props: { label: String, value: [String, Number, null], long: Boolean },
  render() {
    const hasValue = this.value !== null && this.value !== undefined && this.value !== ''
    return h('label', { class: this.long ? 'field long' : 'field' }, [
      h('span', this.label),
      h(
        'p',
        { class: ['read-value', !hasValue && 'is-empty'] },
        hasValue ? String(this.value) : t('Not set'),
      ),
    ])
  },
}

const initials = computed(() => {
  if (!data.value) return ''
  return data.value.name
    .split(' ')
    .filter(Boolean)
    .map((p) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
})

const classLine = computed(() => {
  if (!data.value) return ''
  const parts = [data.value.class_name, data.value.subclass_name, data.value.specialization_name].filter(Boolean)
  return parts.length ? parts.join(' | ') : t('Unclassed')
})

async function load() {
  loading.value = true
  loadError.value = ''
  missing.value = false
  data.value = null
  try {
    data.value = await fetchCurrentVersion(props.id)
    pageTitle.value = data.value.name
  } catch (err) {
    missing.value = await isMissing(`/characters/${props.id}/current`)
    loadError.value = t("Couldn't load this character. Try refreshing.")
  } finally {
    loading.value = false
  }
}

// Reload when a link leads straight to another character (a relation, say).
watch(() => props.id, load, { immediate: true })

// The browser's print dialog, which can also save a PDF. The print styles
// (style.css) show every tab at once on a plain light page.
function print() {
  window.print()
}
</script>

<style scoped>
.group-title {
  margin: 1.1rem 0 0.6rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.group-title:first-of-type {
  margin-top: 0;
}

.empty-toggle {
  margin-top: 0.75rem;
}

.nothing-yet {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.tag-line {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin: 0.35rem 0 0;
}

.char-tag {
  padding: 0.05rem 0.55rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  font-size: 0.72rem;
  color: var(--text-muted);
  text-decoration: none;
}

.char-tag:hover {
  color: var(--accent);
  border-color: color-mix(in srgb, var(--accent) 45%, transparent);
}

.identity-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.identity {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  padding: 1.5rem;
  margin-bottom: 1.75rem;
}

.portrait {
  width: 84px;
  height: 84px;
  border-radius: 14px;
  background: var(--accent-soft);
  overflow: hidden;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.portrait img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.portrait-fallback {
  font-family: 'Fraunces', serif;
  color: var(--accent);
  font-size: 1.5rem;
}

.identity-text {
  flex: 1;
  min-width: 0;
}

.identity-text h1 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.5rem;
  margin: 0;
  color: var(--text-primary);
}

.level {
  font-size: 0.8rem;
  font-family: 'Manrope', sans-serif;
  color: var(--text-faint);
}

.nickname {
  margin: 0.2rem 0 0;
  font-style: italic;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.class-line {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.story-tab,
.build-tab,
.gear-tab-wrap {
  padding: 1.75rem;
}

.long-fields {
  margin-top: 0.5rem;
}

.field.long .read-value {
  white-space: pre-wrap;
  line-height: 1.55;
}

/* Phones: the buttons move under the name. */
/* A label with its value right under it, not a paragraph's gap apart. */
.story-tab :deep(.read-value) {
  margin: 0;
}

.story-tab :deep(.field) {
  margin-bottom: 0.5rem;
}

@media (max-width: 720px) {
  /* Two short fields to a row; the long ones keep the whole width. */
  .story-tab .field-grid {
    grid-template-columns: 1fr 1fr;
    gap: 0.25rem 0.9rem;
  }
  .identity {
    flex-wrap: wrap;
  }
  .identity-text {
    flex: 1;
    min-width: 0;
  }
  .identity-actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
