<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Settings') }}</h1>
        <p class="page-sub">{{ $t('App-wide options that more than one page depends on.') }}</p>
      </div>
    </header>

    <div class="settings-layout">
    <nav class="settings-index" :aria-label="$t('Settings sections')">
      <a
        v-for="s in SECTIONS"
        :key="s.id"
        :href="`#${s.id}`"
        class="index-link"
        :class="{ 'is-active': active === s.id }"
        :aria-current="active === s.id ? 'true' : undefined"
        @click.prevent="openSection(s.id)"
      >{{ $t(s.label) }}</a>
    </nav>

    <div class="settings-main" :class="{ 'is-stats': tab === 'stats' }">
    <!-- Both stay mounted, so unsaved stat edits survive a look at General. -->
    <section id="stats" v-show="tab === 'stats'" class="stats-section" :class="{ 'is-flash': flash === 'stats' }">
      <StatSystemEditor />
    </section>

    <div v-show="tab === 'general'">
    <section id="language" class="glass-panel card" :class="{ 'is-flash': flash === 'language' }">
      <h2>{{ $t('Language') }}</h2>
      <p class="lede">
        {{ $t('The language of the app\'s menus, buttons and messages. What you write in your story is never translated. Remembered on this device only.') }}
      </p>
      <label class="field narrow">
        <span>{{ $t('Interface language') }}</span>
        <select :value="locale" @change="saveLocale($event.target.value)">
          <option v-for="l in LOCALES" :key="l.id" :value="l.id">{{ l.name }}</option>
        </select>
      </label>
    </section>

    <section id="appearance" class="glass-panel card" :class="{ 'is-flash': flash === 'language' }">
      <h2>{{ $t('Appearance') }}</h2>
      <p class="lede">
        {{ $t('Pick a colour theme. It applies straight away and is remembered on this device only, so your phone and your desktop can look different.') }}
      </p>
      <div class="themes" role="radiogroup" :aria-label="$t('Colour theme')">
        <button
          v-for="t in THEMES"
          :key="t.id"
          type="button"
          class="theme-card"
          :class="{ 'is-selected': currentTheme === t.id }"
          :data-theme="t.id"
          role="radio"
          :aria-checked="currentTheme === t.id"
          @click="saveTheme(t.id)"
        >
          <span class="theme-preview">
            <span class="swatch" style="background: var(--accent)"></span>
            <span class="swatch" style="background: var(--logo-star)"></span>
            <span class="swatch" style="background: var(--line)"></span>
            <span class="swatch" style="background: var(--kind-birth)"></span>
            <span class="swatch" style="background: var(--kind-founding)"></span>
          </span>
          <span class="theme-name">{{ t.name }}<span v-if="currentTheme === t.id" class="theme-check">✓</span></span>
          <span class="theme-blurb">{{ $t(t.blurb) }}</span>
        </button>
      </div>

      <h3 id="timeline-rows" class="sub-title">{{ $t('Timeline rows') }}</h3>
      <p class="lede">{{ $t('How the timeline runs from one row to the next. Remembered on this device only.') }}</p>
      <div class="row-styles" role="radiogroup" :aria-label="$t('Timeline rows')">
        <button
          v-for="r in TIMELINE_ROWS"
          :key="r.id"
          type="button"
          class="theme-card row-style"
          :class="{ 'is-selected': timelineRows === r.id }"
          role="radio"
          :aria-checked="timelineRows === r.id"
          @click="timelineRows = r.id"
        >
          <svg viewBox="0 0 120 54" class="row-preview" aria-hidden="true">
            <path v-if="r.id === 'snake'" d="M10 10 H95 A11 11 0 0 1 95 32 H25 A11 11 0 0 0 25 54" />
            <template v-else>
              <path d="M10 10 H110 M10 32 H110 M10 54 H60" />
              <path class="row-return" d="M110 10 C118 10 118 21 110 21 H10 C2 21 2 32 10 32 M110 32 C118 32 118 43 110 43 H10 C2 43 2 54 10 54" />
            </template>
          </svg>
          <span class="theme-name">{{ $t(r.name) }}<span v-if="timelineRows === r.id" class="theme-check">✓</span></span>
          <span class="theme-blurb">{{ $t(r.blurb) }}</span>
        </button>
      </div>
    </section>

    <section id="calendar" class="glass-panel card" :class="{ 'is-flash': flash === 'calendar' }">
      <h2>{{ $t('Calendar') }}</h2>
      <p class="lede">
        {{ $t('How your world counts time. Date inputs, the Q1–Q4 labels and the timeline all follow this, so set it before entering many dates.') }}
      </p>

      <p v-if="loadError" class="error-banner">{{ loadError }}</p>
      <template v-else>
        <div class="field-row narrow">
          <label class="field">
            <span>{{ $t('Months per year') }}</span>
            <input v-model.number="months" type="number" min="1" :max="100" step="1" />
          </label>
          <label class="field">
            <span>{{ $t('Days per month') }}</span>
            <input v-model.number="days" type="number" min="1" :max="400" step="1" />
          </label>
          <label class="field" :title="$t('How long the world takes to turn once. With the months and days, it sets how long its year round the star is (Solar system).')">
            <span>{{ $t('Hours per day') }}</span>
            <input v-model.number="hours" type="number" min="0.01" step="any" />
          </label>
        </div>

        <div class="month-names">
          <span class="quarters-title">{{ $t('Month names') }}</span>
          <p class="note">{{ $t('Optional. A named month reads as words in dates; one left empty keeps its number.') }}</p>
          <div class="month-name-grid">
            <label v-for="m in nameSlots" :key="m" class="month-name">
              <span>{{ m }}</span>
              <input v-model="monthNames[m - 1]" type="text" maxlength="24" :placeholder="$t('Month {n}', { n: m })" />
            </label>
          </div>
        </div>

        <div class="field-row narrow eras">
          <label class="field" :title="$t('Written after a year before 1. Left empty, those years show with a minus sign.')">
            <span>{{ $t('Era before year 1') }}</span>
            <input v-model="yearBefore" type="text" maxlength="24" :placeholder="$t('e.g. BC')" />
          </label>
          <label class="field" :title="$t('Written after every year from 1 on. Usually left empty.')">
            <span>{{ $t('Era from year 1') }}</span>
            <input v-model="yearAfter" type="text" maxlength="24" :placeholder="$t('e.g. AD')" />
          </label>
        </div>
        <p class="note">{{ $t('Dates read like this: {example}.', { example: eraExample }) }}</p>

        <div class="quarters">
          <span class="quarters-title">{{ $t('Quarters') }}</span>
          <ul>
            <li v-for="q in quarterRows" :key="q.q">
              <strong>{{ $t('Q{n}', { n: q.q }) }}</strong>
              <span>{{ q.range }}</span>
            </li>
          </ul>
          <p v-if="uneven" class="note">
            {{ $t('{n} months don\'t split into four equal quarters, so some quarters get one more month than others.', { n: months }) }}
          </p>
        </div>

        <p class="note">
          {{ $t('Changing this doesn\'t rewrite dates you\'ve already saved. A saved date that no longer exists in the new calendar (say, day 31 after switching to 30-day months) is kept as is, and will be refused the next time you save that item until you fix it.') }}
        </p>

        <p v-if="saveError" class="error-banner">{{ saveError }}</p>
        <div class="actions">
          <button class="btn btn-primary" :disabled="saving || !valid" @click="save">
            {{ saving ? $t('Saving…') : $t('Save calendar') }}
          </button>
          <span v-if="saved" class="saved-hint">{{ $t('Saved.') }}</span>
          <span v-else-if="unsaved('calendar')" class="unsaved-hint">{{ $t('Unsaved changes') }}</span>
          <button type="button" class="btn btn-ghost small restore-btn" :disabled="saving" @click="restoring = 'calendar'">{{ $t('Restore default') }}</button>
        </div>
      </template>
    </section>

    <section id="seasons" class="glass-panel card" :class="{ 'is-flash': flash === 'seasons' }">
      <h2>{{ $t('Seasons') }}</h2>
      <p class="lede">
        {{ $t('Name your world\'s seasons, put them in order and say how many months each lasts. They repeat from the day the first one starts, so a round of seasons can be longer than a year: a winter can last three years. The calendar shows them, and regions on the wiki can give temperatures for each.') }}
      </p>

      <template v-if="!loadError">
        <div v-if="seasonList.length" class="cycle-bar" :aria-label="$t('One round of seasons')">
          <span
            v-for="s in seasonList"
            :key="s.id"
            :style="{ flexGrow: s.months > 0 ? s.months : 0.01, background: s.color }"
            :title="`${s.name} · ${$tn('{n} month', '{n} months', s.months)}`"
          >{{ s.name }}</span>
        </div>
        <p v-if="seasonList.length && seasonsValid" class="note cycle-note">{{ cycleNote }}</p>

        <ul class="season-rows">
          <li v-for="(s, i) in seasonList" :key="s.id">
            <input v-model="s.color" type="color" class="season-color" :aria-label="$t('Colour')" />
            <input v-model="s.name" type="text" maxlength="60" class="season-name" :placeholder="$t('Season name')" :aria-label="$t('Season name')" />
            <label class="season-months">
              <input v-model.number="s.months" type="number" min="0.5" max="1200" step="0.5" :aria-label="$t('Months')" />
              <span>{{ $t('months') }}</span>
            </label>
            <button type="button" class="btn btn-ghost small" :disabled="i === 0" :aria-label="$t('Move up')" @click="moveSeason(i, -1)">↑</button>
            <button type="button" class="btn btn-ghost small" :disabled="i === seasonList.length - 1" :aria-label="$t('Move down')" @click="moveSeason(i, 1)">↓</button>
            <button type="button" class="btn btn-danger small" @click="seasonList.splice(i, 1)">{{ $t('Remove') }}</button>
          </li>
        </ul>
        <p v-if="!seasonList.length" class="note">{{ $t('No seasons: the calendar shows none.') }}</p>
        <div class="season-add">
          <button type="button" class="btn btn-ghost small" :disabled="seasonList.length >= 24" @click="addSeason">{{ $t('Add season') }}</button>
        </div>

        <template v-if="seasonList.length">
          <div class="season-fields">
            <DateField v-model="seasonStart" :label="$t('The first season starts on')" />
            <label class="field">
              <span>{{ $t('These seasons are for the') }}</span>
              <select v-model="hemisphere">
                <option value="north">{{ $t('Northern hemisphere') }}</option>
                <option value="south">{{ $t('Southern hemisphere') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ $t('Temperatures in') }}</span>
              <select v-model="unit">
                <option value="C">°C</option>
                <option value="F">°F</option>
              </select>
            </label>
          </div>
          <p class="note">
            {{ $t('The other hemisphere has the same seasons half a round later: its summer falls in your winter. The calendar shows both.') }}
          </p>
        </template>

        <p v-if="seasonError" class="error-banner">{{ seasonError }}</p>
        <div class="actions">
          <button class="btn btn-primary" :disabled="seasonSaving || !seasonsValid" @click="saveSeasonSettings">
            {{ seasonSaving ? $t('Saving…') : $t('Save seasons') }}
          </button>
          <span v-if="seasonSaved" class="saved-hint">{{ $t('Saved.') }}</span>
          <span v-else-if="unsaved('seasons')" class="unsaved-hint">{{ $t('Unsaved changes') }}</span>
          <button type="button" class="btn btn-ghost small restore-btn" :disabled="seasonSaving" @click="restoring = 'seasons'">{{ $t('Restore default') }}</button>
        </div>
      </template>
    </section>

    <section id="map" class="glass-panel card" :class="{ 'is-flash': flash === 'map' }">
      <h2>{{ $t('Map') }}</h2>
      <p class="lede">
        {{ $t('A flat hex map, or a whole planet of hexes you paint on a globe. On a planet, the flat maps are projections of it, and everything that depends on where things are (borders, latitudes, territories) is read from the planet. The 2D map is kept as it is, as a 2D-only version you can go back to or port onto the planet.') }}
      </p>

      <div class="modes" role="radiogroup" :aria-label="$t('Map type')">
        <label class="mode" :class="{ 'is-on': mapForm.mode === '2d' }">
          <input v-model="mapForm.mode" type="radio" value="2d" />
          <strong>{{ $t('2D only') }}</strong>
          <span>{{ $t('A flat map of hexes, as before.') }}</span>
        </label>
        <label class="mode" :class="{ 'is-on': mapForm.mode === '3d' }">
          <input v-model="mapForm.mode" type="radio" value="3d" />
          <strong>{{ $t('3D planet') }}</strong>
          <span>{{ $t('A globe of hexes, with flat projections to look at.') }}</span>
        </label>
      </div>

      <div class="planet-fields">
        <label class="field">
          <span>{{ $t('Planet circumference (km)') }}</span>
          <input v-model.number="mapForm.planet_km" type="number" min="1" step="any" />
        </label>
        <label v-if="isPlanet" class="field">
          <span>{{ $t('Hex width (km)') }}</span>
          <input v-model.number="mapForm.hex_km" type="number" min="0.01" step="any" />
        </label>
        <label class="field">
          <span>{{ $t('Axial tilt (°)') }}</span>
          <input v-model.number="mapForm.tilt" type="number" min="0" max="90" step="any" />
        </label>
        <label v-if="isPlanet" class="field">
          <span>{{ $t('Bestiary territories drawn in') }}</span>
          <select v-model="mapForm.bestiary_projection">
            <option v-for="p in PROJECTIONS" :key="p.id" :value="p.id">{{ $t(p.label) }}</option>
          </select>
        </label>
      </div>
      <p v-if="mapValid && isPlanet" class="note">
        {{ $t('The planet has {cells} hexes, each about {km} km across.', { cells: planetCells.toLocaleString(), km: planetHexKm }) }}
        <template v-if="planetFrequency !== savedFrequency && savedFrequency">
          {{ $t('Changing the hex size moves everything already painted on the planet onto the new hexes.') }}
        </template>
        <strong v-if="planetCells > 300000">{{ $t('That many hexes may be slow to draw.') }}</strong>
      </p>

      <p v-if="mapError" class="error-banner">{{ mapError }}</p>
      <div class="actions">
        <button class="btn btn-primary" :disabled="mapSaving || !mapValid" @click="saveMapSettings">
          {{ mapSaving ? $t('Saving…') : $t('Save map settings') }}
        </button>
        <span v-if="mapSaved" class="saved-hint">{{ $t('Saved.') }}</span>
          <span v-else-if="unsaved('map')" class="unsaved-hint">{{ $t('Unsaved changes') }}</span>
        <button type="button" class="btn btn-ghost small restore-btn" :disabled="mapSaving" @click="restoring = 'map'">{{ $t('Restore default') }}</button>
      </div>

      <div v-if="isPlanet" class="port">
        <h3>{{ $t('Port the 2D map onto the planet') }}</h3>
        <p class="note">
          {{ $t('Reads the 2D map as drawn in a projection, with its equator as the planet\'s and its hexes as wide as the 2D map\'s Equator panel says, and copies every painted hex, place and creature territory onto the planet. Whatever is on the planet now is replaced. The 2D map itself is not changed.') }}
        </p>
        <div class="port-row">
          <label class="field">
            <span>{{ $t('The 2D map is drawn in') }}</span>
            <select v-model="portProjection">
              <option v-for="p in PROJECTIONS" :key="p.id" :value="p.id">{{ $t(p.label) }}</option>
            </select>
          </label>
          <button class="btn btn-ghost" :disabled="porting" @click="portOpen = true">
            {{ porting ? $t('Porting…') : $t('Port the 2D map') }}
          </button>
        </div>
        <p class="note">{{ $t(PROJECTIONS.find((p) => p.id === portProjection)?.blurb || '') }}</p>
        <p v-if="portDone" class="saved-hint">{{ portDone }}</p>
      </div>
      <p v-else class="note">{{ $t('Choose 3D planet to port the 2D map onto the planet.') }}</p>
    </section>

    <section id="sky" class="glass-panel card" :class="{ 'is-flash': flash === 'sky' }">
      <h2>{{ $t('Sky') }}</h2>
      <p class="lede">
        {{ $t('The sky behind the planet in the 3D map, turning with it as you look around. Use your own picture of the sky over your world; the template is the default sky, so it shows the format.') }}
      </p>
      <div class="sky-row">
        <img class="sky-preview" :src="currentSky" :alt="$t('The sky')" />
        <div class="sky-actions">
          <a class="btn btn-ghost small" :href="DEFAULT_SKY" download="polaris-sky-template.jpg">{{ $t('Download the template') }}</a>
          <button type="button" class="btn btn-ghost small" :disabled="skyBusy" @click="skyInput.click()">
            {{ skyBusy ? $t('Saving…') : $t('Upload your sky') }}
          </button>
          <button v-if="worldMap.sky_url" type="button" class="btn btn-ghost small" :disabled="skyBusy" @click="restoring = 'sky'">
            {{ $t('Restore default') }}
          </button>
          <input ref="skyInput" type="file" accept="image/jpeg,image/png" hidden @change="uploadSky" />
        </div>
      </div>
      <ul class="sky-format">
        <li>{{ $t('A JPEG or PNG exactly twice as wide as it is tall, up to 8192 × 4096.') }}</li>
        <li>{{ $t('The sky as seen from the ground, like a photo of it: the top edge is the sky straight above the north pole, the bottom edge above the south pole, the middle row the celestial equator.') }}</li>
        <li>{{ $t('The middle column is above longitude 0; east is to the left. The left and right edges meet, so they must match.') }}</li>
        <li>{{ $t('Near the top and bottom the picture is stretched sideways, as on a world map: draw round things there wider than they are tall.') }}</li>
      </ul>
      <p v-if="skyError" class="error-banner">{{ skyError }}</p>
    </section>

    <section id="blood-types" class="glass-panel card" :class="{ 'is-flash': flash === 'blood-types' }">
      <h2>{{ $t('Blood types') }}</h2>
      <p class="lede">
        {{ $t('The blood types the character form offers, one button each. Add your world\'s own, or remove the ones it doesn\'t have. The note is shown when one is picked; leave it empty on a usual type to show what it is said to mean in Japan.') }}
      </p>

      <ul class="blood-rows">
        <li v-for="(b, i) in bloodList" :key="b.uid">
          <input v-model="b.name" type="text" maxlength="24" class="blood-name" :placeholder="$t('Name')" :aria-label="$t('Name')" />
          <textarea
            v-model="b.note"
            rows="2"
            maxlength="600"
            class="blood-note-input"
            :placeholder="bloodTypeNote({ name: b.name, note: '' }) || $t('A note about this blood type (optional)')"
            :aria-label="$t('Note')"
          ></textarea>
          <button type="button" class="btn btn-danger small" @click="bloodList.splice(i, 1)">{{ $t('Remove') }}</button>
        </li>
      </ul>
      <p v-if="!bloodList.length" class="note">{{ $t('No blood types: the character form offers none.') }}</p>
      <div class="season-add">
        <button type="button" class="btn btn-ghost small" :disabled="bloodList.length >= 64" @click="addBloodType">{{ $t('Add blood type') }}</button>
      </div>

      <p v-if="bloodError" class="error-banner">{{ bloodError }}</p>
      <div class="actions">
        <button class="btn btn-primary" :disabled="bloodSaving || !bloodValid" @click="saveBloodTypeList()">
          {{ bloodSaving ? $t('Saving…') : $t('Save blood types') }}
        </button>
        <span v-if="bloodSaved" class="saved-hint">{{ $t('Saved.') }}</span>
          <span v-else-if="unsaved('blood')" class="unsaved-hint">{{ $t('Unsaved changes') }}</span>
        <button type="button" class="btn btn-ghost small restore-btn" :disabled="bloodSaving" @click="restoring = 'blood'">{{ $t('Restore default') }}</button>
      </div>
    </section>

    <section id="fonts" class="glass-panel card" :class="{ 'is-flash': flash === 'fonts' }">
      <h2>{{ $t('Fonts') }}</h2>
      <p class="lede">
        {{ $t('Your own font files for the writing editor, with a record of each one\'s license. One library for every story.') }}
      </p>
      <router-link to="/fonts" class="btn btn-ghost">{{ $t('Open the fonts library') }}</router-link>
    </section>

    <RestoreDefaultDialog
      v-if="restoring"
      :title="$t(RESTORES[restoring].title)"
      :message="$t(RESTORES[restoring].message)"
      @cancel="restoring = ''"
      @confirm="restore"
    />

    <ConfirmDialog
      v-if="portOpen"
      :title="$t('Port the 2D map onto the planet?')"
      :message="$t('Everything painted on the planet, and where its places stand, is replaced by the 2D map.')"
      :confirm-label="$t('Port the 2D map')"
      @cancel="portOpen = false"
      @confirm="portMap"
    />

    <section id="about" class="glass-panel card about" :class="{ 'is-flash': flash === 'about' }">
      <h2>{{ $t('About Polaris') }}</h2>
      <!-- The Appropriate Legal Notices (AGPL-3.0 and NOTICE.md): a changed
           version keeps "Based on Polaris by AstraLumi" with its link here,
           and points Source code at its own source. -->
      <dl class="about-list">
        <dt>{{ $t('Version') }}</dt>
        <dd>{{ appVersion }}</dd>
        <dt>{{ $t('Made by') }}</dt>
        <dd><a :href="REPO_OWNER_URL" target="_blank" rel="noopener noreferrer">AstraLumi</a></dd>
        <dt>{{ $t('License') }}</dt>
        <dd>
          <a :href="`${REPO_URL}/blob/main/LICENSE`" target="_blank" rel="noopener noreferrer">AGPL-3.0</a>
          {{ $t('with') }}
          <a :href="`${REPO_URL}/blob/main/NOTICE.md`" target="_blank" rel="noopener noreferrer">{{ $t('additional terms') }}</a>
          · {{ $t('free to use, change and share; a changed version that is shared or hosted for others shares its source too; provided as is, without warranty') }}
        </dd>
        <dt>{{ $t('Source code') }}</dt>
        <dd><a :href="REPO_URL" target="_blank" rel="noopener noreferrer">github.com/AstraLumi/polaris</a></dd>
      </dl>
    </section>
    </div>
    </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useUnsavedGuard, useSaveShortcut } from '../navigation'
import { timelineRows } from '../timelinePrefs'
import { dayText, yearText } from '../storyDates'
import { fetchSettings, saveSettings, saveSeasons, saveBloodTypes, planetApi } from '../api'
import RestoreDefaultDialog from '../components/RestoreDefaultDialog.vue'
import { applyBloodTypes, bloodTypeNote, DEFAULT_BLOOD_TYPES } from '../bloodTypes'
import { loadCalendar, calendar } from '../calendar'
import { THEMES, currentTheme, saveTheme } from '../theme'
import { t, tn, tr, LOCALES, locale, saveLocale } from '../i18n'
import pkg from '../../package.json'
import { useRoute, useRouter } from 'vue-router'
import StatSystemEditor from '../components/StatSystemEditor.vue'
import DateField from '../components/DateField.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { PROJECTIONS, applyMapSettings, worldMap, DEFAULT_SKY, skyUrl } from '../worldMap'
import { frequencyFor, hexKmOf } from '../planetGrid'
import { newSeasonId } from '../seasons'

// The side index. Stats is a tab of its own (a big editor that stays mounted),
// the rest are sections of the General page; `ids` are the cards a label covers.
const SECTIONS = [
  { id: 'language', label: tr('Language & look'), ids: ['language', 'appearance', 'timeline-rows'] },
  { id: 'calendar', label: tr('Calendar'), ids: ['calendar'] },
  { id: 'seasons', label: tr('Seasons'), ids: ['seasons'] },
  { id: 'map', label: tr('Map'), ids: ['map'] },
  { id: 'sky', label: tr('Sky'), ids: ['sky'] },
  { id: 'blood-types', label: tr('Blood types'), ids: ['blood-types'] },
  { id: 'fonts', label: tr('Fonts'), ids: ['fonts'] },
  { id: 'stats', label: tr('Stats'), ids: ['stats'] },
  { id: 'about', label: tr('About Polaris'), ids: ['about'] },
]
const sectionOf = (id) => SECTIONS.find((s) => s.ids.includes(id))?.id

// /settings#calendar scrolls to a section; ?tab=stats (older links) and
// #stats open the Stats tab.
const route = useRoute()
const router = useRouter()
const tab = ref(route.query.tab === 'stats' || route.hash === '#stats' ? 'stats' : 'general')
const active = ref(tab.value === 'stats' ? 'stats' : 'language')
const flash = ref('')
let flashTimer = 0
let jumping = 0

async function goTo(id) {
  const key = sectionOf(id)
  if (!key) return
  tab.value = key === 'stats' ? 'stats' : 'general'
  active.value = key
  await nextTick()
  clearTimeout(flashTimer)
  flash.value = key
  flashTimer = setTimeout(() => (flash.value = ''), 1400)
  // The scroll spy stays quiet while the smooth scroll runs.
  jumping = Date.now()
  if (key === 'stats') window.scrollTo({ top: 0, behavior: 'smooth' })
  else document.getElementById(id)?.scrollIntoView({ block: 'start', behavior: 'smooth' })
}

function openSection(id) {
  if (route.hash === `#${id}`) goTo(id)
  else router.replace({ path: route.path, hash: `#${id}` })
}

watch(() => route.hash, (h) => h && goTo(h.slice(1)))

// Highlight the section being read: the last one whose top has passed the
// top of the page.
function spy() {
  if (tab.value === 'stats' || Date.now() - jumping < 900) return
  let found = SECTIONS[0].id
  for (const s of SECTIONS) {
    if (s.id === 'stats') continue
    const el = document.getElementById(s.ids[0])
    if (el && el.getBoundingClientRect().top <= 140) found = s.id
  }
  const bottom = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4
  active.value = bottom ? 'about' : found
}

// The version comes from package.json at build time, so it is always the
// one that was built.
const appVersion = `v${pkg.version}`
const REPO_URL = 'https://github.com/AstraLumi/polaris'
const REPO_OWNER_URL = 'https://github.com/AstraLumi'

// The timeline's two ways of turning from one row to the next.
const TIMELINE_ROWS = [
  { id: 'snake', name: tr('Back and forth'), blurb: tr('The line turns at the end of each row and comes back, so every other row runs right to left. The default.') },
  { id: 'ltr', name: tr('Left to right'), blurb: tr('Every row runs left to right, like lines of text; a faint line leads back to the next row.') },
]

// What each section held when last loaded or saved, so edits not yet
// saved are noticed: a note beside its Save button, and a question before
// leaving the page.
const savedState = ref({})
const sectionState = {
  calendar: () => JSON.stringify([months.value, days.value, hours.value, yearBefore.value.trim(), yearAfter.value.trim(), namesToSave()]),
  seasons: () => JSON.stringify([seasonList.value, seasonStart.value, hemisphere.value, unit.value]),
  map: () => JSON.stringify(mapForm.value),
  blood: () => JSON.stringify(bloodList.value.map((b) => [b.name, b.note])),
}
function markSaved(key) {
  savedState.value = { ...savedState.value, [key]: sectionState[key]() }
}
const unsaved = (key) => key in savedState.value && sectionState[key]() !== savedState.value[key]
useUnsavedGuard(() => Object.keys(sectionState).some(unsaved))
// Ctrl+S saves every section with unsaved changes (Stats has its own).
useSaveShortcut(() => {
  if (tab.value === 'stats') return
  if (unsaved('calendar') && !saving.value) save()
  if (unsaved('seasons') && seasonsValid.value && !seasonSaving.value) saveSeasonSettings()
  if (unsaved('map') && mapValid.value && !mapSaving.value) saveMapSettings()
  if (unsaved('blood') && bloodValid.value && !bloodSaving.value) saveBloodTypeList()
})

const months = ref(calendar.months_per_year)
// One box per month, following Months per year. Names typed for months
// past the last are kept (here and on the server) in case the year grows
// again.
const monthNames = ref(takeNames(calendar.month_names))
const nameSlots = computed(() => Math.min(100, Math.max(1, Number.isInteger(months.value) ? months.value : 1)))
function takeNames(list) {
  return Array.from({ length: 100 }, (_, i) => (list || [])[i] || '')
}
function namesToSave() {
  const names = monthNames.value.map((n) => (n || '').trim())
  while (names.length && !names[names.length - 1]) names.pop()
  return names
}
const yearBefore = ref(calendar.year_before)
const yearAfter = ref(calendar.year_after)
// How dates will read with the era names typed (storyDates.js).
const eraExample = computed(() => {
  const eras = { before: yearBefore.value.trim(), after: yearAfter.value.trim() }
  const names = namesToSave().slice(0, nameSlots.value)
  return [dayText({ day: 15, month: 3, year: 1054 }, false, eras, names), dayText({ day: 1, month: 1, year: -54 }, false, eras, names), `${yearText(1054, eras)} ${t('Q{n}', { n: 2 })}`].join(' · ')
})
const days = ref(calendar.days_per_month)
const hours = ref(calendar.hours_per_day)
const loadError = ref('')
const saveError = ref('')
const saving = ref(false)
const saved = ref(false)

const valid = computed(
  () =>
    Number.isInteger(months.value) && months.value >= 1 && months.value <= 100 &&
    Number.isInteger(days.value) && days.value >= 1 && days.value <= 400 &&
    hours.value > 0 && hours.value <= 100000,
)
const uneven = computed(() => valid.value && months.value % 4 !== 0)

const named = (m) => (monthNames.value[m - 1] || '').trim()

// Which months fall in each quarter, using the same rule as quarterOf().
const quarterRows = computed(() => {
  if (!valid.value) return []
  const rows = []
  for (let q = 1; q <= 4; q++) {
    // Same rule as quarterOf(): which months land in this quarter.
    let lo = null
    let hi = null
    for (let m = 1; m <= months.value; m++) {
      if (Math.floor(((m - 1) * 4) / months.value) + 1 === q) {
        if (lo === null) lo = m
        hi = m
      }
    }
    rows.push({
      q,
      range:
        lo === null
          ? t('(no months)')
          : named(lo) && named(hi)
            ? lo === hi ? named(lo) : `${named(lo)} – ${named(hi)}`
            : lo === hi
              ? t('month {n}', { n: lo })
              : t('months {from}–{to}', { from: lo, to: hi }),
    })
  }
  return rows
})

// ---- Seasons ---------------------------------------------------------------------
const seasonList = ref([])
const seasonStart = ref('')
const hemisphere = ref('north')
const unit = ref('C')
const seasonError = ref('')
const seasonSaving = ref(false)
const seasonSaved = ref(false)

const SEASON_COLORS = ['#7fb2e5', '#86cf7f', '#f2c14e', '#e0864a', '#b48ce0', '#e57f9e']

function takeSeasons(s) {
  seasonList.value = (s.seasons || []).map((x, i) => ({ ...x, color: x.color || SEASON_COLORS[i % SEASON_COLORS.length] }))
  seasonStart.value = s.start || ''
  hemisphere.value = s.hemisphere || 'north'
  unit.value = s.unit || 'C'
  markSaved('seasons')
}

const seasonsValid = computed(
  () =>
    seasonList.value.every((s) => s.name.trim() && s.months > 0 && s.months <= 1200) &&
    (!seasonList.value.length || /^(\d{1,3}-\d{1,3}-)?-?\d+$/.test(seasonStart.value || '')),
)

// How long one round of seasons is, against the year.
const cycleNote = computed(() => {
  const total = Math.round(seasonList.value.reduce((n, s) => n + s.months, 0) * 100) / 100
  const perYear = calendar.months_per_year
  if (total === perYear) return t('One round of seasons is one year.')
  if (total % perYear === 0) return t('One round of seasons is {n} years.', { n: total / perYear })
  if (total > perYear) {
    return t('One round of seasons is {n} months, about {years} years.', { n: total, years: Math.round((total / perYear) * 100) / 100 })
  }
  return tn('One round of seasons is {n} month, not a whole year.', 'One round of seasons is {n} months, not a whole year.', total)
})

function moveSeason(i, by) {
  const list = seasonList.value
  ;[list[i], list[i + by]] = [list[i + by], list[i]]
}

function addSeason() {
  seasonList.value.push({ id: newSeasonId(), name: '', months: 3, color: SEASON_COLORS[seasonList.value.length % SEASON_COLORS.length] })
}

// Winter, spring, summer and fall, a quarter of the year each, winter from
// the last month. The ids are the defaults', so temperatures typed for
// them come back.
function resetSeasons() {
  const quarter = Math.round((calendar.months_per_year / 4) * 100) / 100
  seasonList.value = [
    ['winter', t('Winter')],
    ['spring', t('Spring')],
    ['summer', t('Summer')],
    ['fall', t('Fall')],
  ].map(([id, name], i) => ({ id, name, months: quarter, color: SEASON_COLORS[i] }))
  seasonStart.value = `01-${String(calendar.months_per_year).padStart(2, '0')}-0001`
}

async function saveSeasonSettings() {
  seasonSaving.value = true
  seasonError.value = ''
  seasonSaved.value = false
  try {
    const s = await saveSeasons({
      seasons: seasonList.value.map((x) => ({ ...x, name: x.name.trim() })),
      start: seasonStart.value,
      hemisphere: hemisphere.value,
      unit: unit.value,
    })
    takeSeasons(s)
    await loadCalendar()
    seasonSaved.value = true
    setTimeout(() => (seasonSaved.value = false), 2500)
  } catch (err) {
    seasonError.value = err.message || t("Couldn't save.")
  } finally {
    seasonSaving.value = false
  }
}

// ---- Map: 2D only or a planet ----------------------------------------------------------
const mapForm = ref({ mode: '2d', planet_km: 40075, hex_km: 50, tilt: 23.44, bestiary_projection: 'equirectangular', show_equator: true, show_zones: false })
const savedFrequency = ref(0)
const mapError = ref('')
const mapSaving = ref(false)
const mapSaved = ref(false)
const portProjection = ref('equirectangular')
const portOpen = ref(false)
const porting = ref(false)
const portDone = ref('')

function takeMap(m) {
  if (!m) return
  mapForm.value = {
    mode: m.mode,
    planet_km: m.planet.planet_km,
    hex_km: Math.round(m.hex_km * 100) / 100,
    tilt: m.planet.tilt,
    bestiary_projection: m.planet.bestiary_projection,
    show_equator: m.planet.show_equator,
    show_zones: m.planet.show_zones,
  }
  savedFrequency.value = m.planet.frequency
  markSaved('map')
}

const isPlanet = computed(() => mapForm.value.mode === '3d')
const mapValid = computed(() => {
  const f = mapForm.value
  return f.planet_km > 0 && f.hex_km > 0 && f.tilt >= 0 && f.tilt <= 90
})
const planetFrequency = computed(() => (mapValid.value ? frequencyFor(mapForm.value.planet_km, mapForm.value.hex_km) : 0))
const planetCells = computed(() => 10 * planetFrequency.value ** 2 + 2)
const planetHexKm = computed(() => Math.round(hexKmOf(mapForm.value.planet_km, planetFrequency.value) * 10) / 10)

async function saveMapSettings() {
  mapSaving.value = true
  mapError.value = ''
  mapSaved.value = false
  try {
    const m = await planetApi.saveSettings(mapForm.value)
    takeMap(m)
    applyMapSettings(m)
    mapSaved.value = true
    setTimeout(() => (mapSaved.value = false), 2500)
  } catch (err) {
    mapError.value = err.message || t("Couldn't save.")
  } finally {
    mapSaving.value = false
  }
}

// The sky (backend/sky.go): saved as soon as it is picked.
const skyInput = ref(null)
const skyBusy = ref(false)
const skyError = ref('')
const currentSky = computed(() => skyUrl())

async function uploadSky(e) {
  const file = e.target.files[0]
  e.target.value = ''
  if (!file) return
  skyBusy.value = true
  skyError.value = ''
  try {
    applyMapSettings(await planetApi.uploadSky(file))
  } catch (err) {
    skyError.value = err.message
  } finally {
    skyBusy.value = false
  }
}

async function resetSky() {
  skyBusy.value = true
  skyError.value = ''
  try {
    applyMapSettings(await planetApi.resetSky())
  } catch (err) {
    skyError.value = err.message
  } finally {
    skyBusy.value = false
  }
}

async function portMap() {
  portOpen.value = false
  porting.value = true
  portDone.value = ''
  mapError.value = ''
  try {
    const res = await planetApi.port(portProjection.value)
    portDone.value = tn('Ported: {n} hex painted on the planet.', 'Ported: {n} hexes painted on the planet.', res.cells)
  } catch (err) {
    mapError.value = err.message || t("Couldn't port the map.")
  } finally {
    porting.value = false
  }
}

// ---- Blood types ----------------------------------------------------------------
const bloodList = ref([])
const bloodError = ref('')
const bloodSaving = ref(false)
const bloodSaved = ref(false)
let bloodUid = 0

function takeBloodTypes(list) {
  bloodList.value = (list || []).map((b) => ({ uid: ++bloodUid, name: b.name, note: b.note || '' }))
  markSaved('blood')
}

const bloodValid = computed(() => {
  const names = bloodList.value.map((b) => b.name.trim().toLowerCase())
  return names.every(Boolean) && new Set(names).size === names.length
})

function addBloodType() {
  bloodList.value.push({ uid: ++bloodUid, name: '', note: '' })
}

async function saveBloodTypeList() {
  bloodSaving.value = true
  bloodError.value = ''
  bloodSaved.value = false
  try {
    const list = await saveBloodTypes(bloodList.value.map((b) => ({ name: b.name.trim(), note: b.note.trim() })))
    takeBloodTypes(list)
    applyBloodTypes(list)
    bloodSaved.value = true
    setTimeout(() => (bloodSaved.value = false), 2500)
  } catch (err) {
    bloodError.value = err.message || t("Couldn't save.")
  } finally {
    bloodSaving.value = false
  }
}

// ---- Restore default ------------------------------------------------------------
// Each setting with customizations can go back to how Polaris ships it, once
// the confirmation word is typed (RestoreDefaultDialog). It is saved at once.
const restoring = ref('')
const RESTORES = {
  calendar: {
    title: tr('Restore the default calendar?'),
    message: tr('The calendar goes back to 12 months of 30 days of 24 hours, with no month or era names, and is saved. Dates already saved are not rewritten.'),
    run: () => {
      months.value = 12
      days.value = 30
      hours.value = 24
      yearBefore.value = ''
      yearAfter.value = ''
      monthNames.value = takeNames([])
      return save()
    },
  },
  seasons: {
    title: tr('Restore the default seasons?'),
    message: tr('Your seasons are replaced by winter, spring, summer and fall, a quarter of the year each, and saved.'),
    run: () => {
      resetSeasons()
      hemisphere.value = 'north'
      unit.value = 'C'
      return saveSeasonSettings()
    },
  },
  map: {
    title: tr('Restore the default map settings?'),
    message: tr('The planet goes back to the size of the Earth, with 50 km hexes and an axial tilt of 23.44°, and is saved. What was painted on the planet moves onto the new hexes. The map type is kept.'),
    run: () => {
      mapForm.value = { ...mapForm.value, planet_km: 40075, hex_km: 50, tilt: 23.44, bestiary_projection: 'equirectangular', show_equator: true, show_zones: false }
      return saveMapSettings()
    },
  },
  sky: {
    title: tr('Restore the default sky?'),
    message: tr('Your sky picture is removed and the default sky is used again.'),
    run: () => resetSky(),
  },
  blood: {
    title: tr('Restore the default blood types?'),
    message: tr('The list goes back to A, B, AB and O, each Rh+ and Rh−, with the built-in notes, and is saved. Characters keep the blood type they have.'),
    run: () => {
      takeBloodTypes(DEFAULT_BLOOD_TYPES)
      return saveBloodTypeList()
    },
  },
}

function restore() {
  const which = restoring.value
  restoring.value = ''
  RESTORES[which]?.run()
}

async function save() {
  saving.value = true
  saveError.value = ''
  saved.value = false
  try {
    await saveSettings({
      months_per_year: months.value,
      days_per_month: days.value,
      hours_per_day: hours.value,
      year_before: yearBefore.value.trim(),
      year_after: yearAfter.value.trim(),
      month_names: namesToSave(),
    })
    await loadCalendar()
    markSaved('calendar')
    saved.value = true
    setTimeout(() => (saved.value = false), 2500)
  } catch (err) {
    saveError.value = err.message || t("Couldn't save.")
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    const s = await fetchSettings()
    months.value = s.months_per_year
    days.value = s.days_per_month
    hours.value = s.hours_per_day || 24
    yearBefore.value = s.year_before || ''
    yearAfter.value = s.year_after || ''
    monthNames.value = takeNames(s.month_names)
    markSaved('calendar')
    if (s.seasons) takeSeasons(s.seasons)
    takeMap(s.map)
    takeBloodTypes(s.blood_types)
  } catch (err) {
    loadError.value = "Couldn't load settings. Try refreshing."
  }
  window.addEventListener('scroll', spy, { passive: true })
  if (route.hash && route.hash !== '#stats') {
    await nextTick()
    document.getElementById(route.hash.slice(1))?.scrollIntoView({ block: 'start' })
    active.value = sectionOf(route.hash.slice(1)) || active.value
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', spy)
  clearTimeout(flashTimer)
})
</script>

<style scoped>
.settings-layout {
  display: grid;
  grid-template-columns: 11rem minmax(0, 1fr);
  gap: 1.75rem;
  align-items: start;
}

.settings-index {
  position: sticky;
  top: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  padding-left: 0.25rem;
  border-left: 1px solid var(--glass-border);
}

.index-link {
  margin-left: -1px;
  padding: 0.4rem 0.75rem;
  border-left: 2px solid transparent;
  color: var(--text-muted);
  font-size: 0.88rem;
  font-weight: 600;
  text-decoration: none;
}

.index-link:visited {
  color: var(--text-muted);
}

.index-link:hover,
.index-link.is-active:visited,
.index-link:hover:visited {
  color: var(--text-primary);
}

.index-link.is-active {
  color: var(--text-primary);
  border-left-color: var(--accent);
}

/* A section flashes briefly when the index or a link jumps to it. */
.is-flash {
  animation: section-flash 1.4s ease;
}

@keyframes section-flash {
  0%, 60% { box-shadow: 0 0 0 3px var(--accent-soft); }
  100% { box-shadow: 0 0 0 3px transparent; }
}

.settings-main section {
  scroll-margin-top: 1rem;
}

@media (max-width: 720px) {
  .settings-layout {
    display: block;
  }
  /* A strip of chips that scrolls on its own, so the page never does. */
  .settings-index {
    position: sticky;
    top: 3.9rem;
    z-index: 5;
    flex-direction: row;
    gap: 0.4rem;
    margin: 0 0 1rem;
    padding: 0.5rem 0;
    overflow-x: auto;
    border-left: 0;
    background: var(--bg);
    scrollbar-width: none;
  }
  .index-link {
    flex: 0 0 auto;
    margin: 0;
    padding: 0.4rem 0.8rem;
    border: 1px solid var(--glass-border);
    border-radius: 999px;
    white-space: nowrap;
  }
  .index-link.is-active {
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .settings-main section {
    scroll-margin-top: 7.5rem;
  }
}

.about-list {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.45rem 1.25rem;
  margin: 0.75rem 0 0;
  font-size: 0.88rem;
}

.about-list dt {
  color: var(--text-muted);
}

.about-list dd {
  margin: 0;
  color: var(--text-primary);
}

.about-list a {
  color: var(--accent);
}

@media (max-width: 720px) {
  .about-list {
    grid-template-columns: 1fr;
    gap: 0.15rem;
  }
  .about-list dd {
    margin-bottom: 0.5rem;
  }
}

.card {
  padding: 1.75rem;
  margin-bottom: 1.25rem;
}

.themes {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 0.75rem;
}

/* Each card carries its own data-theme, so it shows that theme's colours
   whatever theme is active. */
.theme-card {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  align-items: flex-start;
  text-align: left;
  padding: 0.9rem;
  border-radius: 12px;
  border: 1px solid var(--glass-border);
  background: var(--bg);
  color: var(--text-primary);
  font-family: inherit;
  cursor: pointer;
  transition: transform 0.12s ease, border-color 0.12s ease;
}

.sub-title {
  margin: 1.5rem 0 0.3rem;
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.05rem;
}

.row-styles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(13rem, 1fr));
  gap: 0.75rem;
}

.row-preview {
  width: 100%;
  max-width: 9rem;
  height: auto;
  overflow: visible;
}

.row-preview path {
  fill: none;
  stroke: var(--line);
  stroke-width: 3;
  stroke-linecap: round;
}

.row-preview .row-return {
  stroke-width: 1.2;
  stroke-dasharray: 2 3;
  opacity: 0.5;
}

.theme-card:hover {
  transform: translateY(-1px);
}

.theme-card.is-selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.theme-preview {
  display: flex;
  gap: 0.3rem;
}

.swatch {
  width: 1.1rem;
  height: 1.1rem;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.18);
}

.theme-name {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 700;
  font-size: 0.92rem;
}

.theme-check {
  color: var(--accent);
}

.theme-blurb {
  font-size: 0.76rem;
  line-height: 1.4;
  color: var(--text-muted);
}

.card h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.2rem;
  margin: 0 0 0.5rem;
}

.lede {
  margin: 0 0 1.25rem;
  color: var(--text-muted);
  font-size: 0.9rem;
  line-height: 1.55;
}

.narrow {
  max-width: 420px;
}

.quarters {
  margin: 0.5rem 0 1.25rem;
}

.month-names {
  margin: 0.75rem 0 1rem;
}

.month-names .note {
  margin: 0.2rem 0 0.5rem;
}

.month-name-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
  gap: 0.4rem 0.6rem;
}

.month-name {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.month-name span {
  min-width: 1.6rem;
  text-align: right;
  font-size: 0.78rem;
  color: var(--text-faint);
  font-variant-numeric: tabular-nums;
}

.month-name input {
  flex: 1;
  min-width: 0;
}

.quarters-title {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.quarters ul {
  list-style: none;
  margin: 0.5rem 0 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 0.5rem;
}

.quarters li {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.quarters li strong {
  color: var(--accent);
}

.cycle-bar {
  display: flex;
  overflow: hidden;
  margin-bottom: 0.4rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
}

.cycle-bar span {
  flex-basis: 0;
  min-width: 0;
  padding: 0.35rem 0.5rem;
  overflow: hidden;
  color: #10131c;
  font-size: 0.76rem;
  font-weight: 700;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.cycle-note {
  margin-top: 0;
}

.season-rows {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin: 0.75rem 0 0.6rem;
  padding: 0;
  list-style: none;
}

.season-rows li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}

.season-color {
  width: 2.4rem;
  height: 2.2rem;
  padding: 0.15rem;
  cursor: pointer;
}

.season-name {
  flex: 1 1 10rem;
  min-width: 0;
}

.season-months {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.season-months input {
  width: 5.5rem;
}

.season-add {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1.1rem;
}

.modes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 0.6rem;
  margin-bottom: 1rem;
}

.mode {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.75rem 0.9rem;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  cursor: pointer;
}

.mode:focus-within {
  outline: 2px solid var(--accent-soft);
}

.mode.is-on {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.mode input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.mode strong {
  font-size: 0.92rem;
}

.mode span {
  font-size: 0.78rem;
  line-height: 1.45;
  color: var(--text-muted);
}

.planet-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 0.75rem;
  align-items: end;
}

.port {
  margin-top: 1.5rem;
  padding-top: 1.2rem;
  border-top: 1px solid var(--glass-border);
}

.port h3 {
  margin: 0 0 0.4rem;
  font-size: 1rem;
}

.sky-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.6rem;
}

.sky-preview {
  width: 280px;
  max-width: 100%;
  aspect-ratio: 2 / 1;
  object-fit: cover;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: #05070c;
}

.sky-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.45rem;
}

.sky-format {
  margin: 0 0 0.6rem;
  padding-left: 1.1rem;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-muted);
}

.port-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
}

.port-row .field {
  flex: 0 1 240px;
}

.season-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 0.75rem;
  align-items: end;
}

.note {
  margin: 0.75rem 0 1.25rem;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-faint);
}

.actions {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}

.saved-hint {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.restore-btn {
  margin-left: auto;
}

.blood-rows {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 0 0 0.6rem;
  padding: 0;
  list-style: none;
}

.blood-rows li {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 0.4rem;
}

.blood-name {
  flex: 0 0 6.5rem;
  font-weight: 700;
}

.blood-note-input {
  flex: 1 1 14rem;
  min-width: 0;
  font-size: 0.82rem;
  resize: vertical;
}
</style>
