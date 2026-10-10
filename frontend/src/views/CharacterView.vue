<template>
  <div class="page">
    <BackLink to="/characters" :label="$t('← Characters')" />

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else-if="data">
      <header class="identity glass-panel">
        <div class="portrait">
          <img v-if="data.picture_path" :src="data.picture_path" :alt="data.name" class="zoomable" @click="viewPicture(data.picture_path, data.name)" />
          <span v-else class="portrait-fallback">{{ initials }}</span>
        </div>
        <div class="identity-text">
          <h1>
            {{ data.name }} <span class="level">{{ $t('Lv.') }} {{ String(data.level).padStart(2, '0') }}</span>
            <span v-if="data.story.status && data.story.status !== 'alive'" class="status-badge" :class="'is-' + data.story.status">
              {{ statusLabel(data.story.status) }}
            </span>
          </h1>
          <p v-if="data.nickname" class="nickname">"{{ data.nickname }}"</p>
          <p class="class-line">
            <span v-if="data.class_icon" class="class-icon"><IconImage :src="data.class_icon" :name="data.class_name" /></span>
            {{ classLine }}
          </p>
        </div>
        <div class="identity-actions">
          <RouterLink :to="{ path: '/timeline', query: { person: data.character_id } }" class="btn btn-ghost">
            {{ $t('Timeline') }}
          </RouterLink>
          <RouterLink :to="`/characters/${data.character_id}/versions`" class="btn btn-ghost">
            {{ $t('Edit') }}
          </RouterLink>
          <button type="button" class="btn btn-ghost" :title="$t('Print the whole sheet, or save it as a PDF')" @click="print">
            {{ $t('Print') }}
          </button>
        </div>
      </header>

      <div class="tab-bar">
        <button class="tab-button" :class="{ 'is-active': tab === 'story' }" @click="tab = 'story'">
          {{ $t('Story') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'build' }" @click="tab = 'build'">
          {{ $t('Build') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'gear' }" @click="tab = 'gear'">
          {{ $t('Gear') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'spells' }" @click="tab = 'spells'">
          {{ $t('Spells') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'relations' }" @click="tab = 'relations'">
          {{ $t('Relations') }}
        </button>
      </div>

      <section v-show="tab === 'story'" class="story-tab glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Story') }}</h2>
        <div class="field-grid">
          <ReadField :label="$t('Gender')" :value="data.story.gender" />
          <ReadField :label="$t('Race')" :value="data.story.race_name" />
          <ReadField :label="$t('Height (cm)')" :value="data.story.height" />
          <ReadField :label="$t('Weight (kg)')" :value="data.story.weight" />
          <ReadField :label="$t('Body type')" :value="data.story.body_type_name" />
          <ReadField :label="$t('Age')" :value="data.story.age" />
          <ReadField :label="$t('Blood type')" :value="data.story.blood_type" />
          <ReadField :label="$t('Born in')" :value="data.story.born_in_name || data.story.born_in" />
          <ReadField :label="$t('Nation')" :value="data.story.nation_name || data.story.nation" />
          <ReadField :label="$t('Birth date (in-story)')" :value="data.story.birth_date" />
          <ReadField :label="$t('Human birth date')" :value="data.story.human_birth_date" />
          <ReadField :label="$t('Status')" :value="statusLabel(data.story.status)" />
          <ReadField :label="$t('Deaths')" :value="data.story.deaths" />
        </div>
        <div class="long-fields">
          <ReadField :label="$t('Description')" :value="data.story.description" long />
          <ReadField :label="$t('Bio')" :value="data.story.bio" long />
          <ReadField :label="$t('Speech mannerisms')" :value="data.story.speech_mannerisms" long />
        </div>
      </section>

      <section v-show="tab === 'build'" class="build-tab glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Build') }}</h2>
        <BuildStatsPanel
          :model-value="data.build"
          :computed="data.computed"
          :special-bases="data.special_bases"
          :level="data.level"
          :editable="false"
        />
      </section>

      <section v-show="tab === 'gear'" class="gear-tab-wrap glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Gear') }}</h2>
        <GearTab :model-value="data.gear || {}" :carry-limit="data.computed?.special?.carry_limit || 0" />
      </section>

      <section v-show="tab === 'spells'" class="gear-tab-wrap glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Spells') }}</h2>
        <SpellsTab :model-value="data.spells || []" />
      </section>

      <section v-show="tab === 'relations'" class="gear-tab-wrap glass-panel sheet-part">
        <h2 class="print-only">{{ $t('Relations') }}</h2>
        <RelationsTab :character-id="data.character_id" :character-name="data.name" />
      </section>
    </template>
  </div>
</template>

<script setup>
import IconImage from '../components/IconImage.vue'
import BackLink from '../components/BackLink.vue'
import { statusLabel } from '../characterStatus'
import { pageTitle, viewPicture } from '../navigation'
import { ref, computed, watch, h } from 'vue'
import { t } from '../i18n'
import { fetchCurrentVersion } from '../api'
import BuildStatsPanel from '../components/BuildStatsPanel.vue'
import GearTab from '../components/GearTab.vue'
import SpellsTab from '../components/SpellsTab.vue'
import RelationsTab from '../components/RelationsTab.vue'

const props = defineProps({ id: { type: String, required: true } })

const data = ref(null)
const loading = ref(true)
const loadError = ref('')
const tab = ref('story')

// Tiny inline component for a label + read-only value pair, so the Story
// tab's markup above stays readable instead of repeating this block 14 times.
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
  try {
    data.value = await fetchCurrentVersion(props.id)
    pageTitle.value = data.value.name
  } catch (err) {
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
@media (max-width: 720px) {
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
