<template>
  <div class="page">
    <BackLink :to="`/characters/${id}/versions`" :label="$t('← Back to versions')" />

    <header class="page-header">
      <div>
        <h1>{{ $t('Compare versions') }}</h1>
        <p class="page-sub">{{ $t('What changed between two points in the story.') }}</p>
      </div>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="!versions.length" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <section class="pickers glass-panel panel-solid">
        <label class="picker">
          <span>{{ $t('From') }}</span>
          <select :value="aId" @change="pick('a', $event.target.value)">
            <option v-for="v in versions" :key="v.id" :value="String(v.id)">{{ versionLabel(v) }}</option>
          </select>
        </label>
        <button type="button" class="btn btn-ghost small" :title="$t('Swap')" @click="swap">⇄</button>
        <label class="picker">
          <span>{{ $t('To') }}</span>
          <select :value="bId" @change="pick('b', $event.target.value)">
            <option v-for="v in versions" :key="v.id" :value="String(v.id)">{{ versionLabel(v) }}</option>
          </select>
        </label>
        <label class="only-changes">
          <input v-model="onlyChanges" type="checkbox" />
          <span>{{ $t('Only show changes') }}</span>
        </label>
      </section>

      <p v-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>
      <template v-else-if="a && b">
        <p v-if="aId === bId" class="note">{{ $t('Pick two different versions to see what changed.') }}</p>
        <p v-else-if="!changeCount" class="note">{{ $t('These two versions are identical.') }}</p>
        <p v-else class="note">{{ $tn('{n} change', '{n} changes', changeCount) }}</p>

        <section v-for="group in shownGroups" :key="group.title" class="group glass-panel">
          <h2>{{ group.title }}</h2>
          <table class="diff">
            <tbody>
              <tr v-for="row in group.rows" :key="row.label" :class="{ 'is-changed': row.changed }">
                <th scope="row">{{ row.label }}</th>
                <td class="from" :class="{ long: row.long }">{{ show(row.from) }}</td>
                <td class="arrow">{{ row.changed ? '→' : '' }}</td>
                <td class="to" :class="{ long: row.long }">
                  {{ show(row.to) }}
                  <span v-if="row.delta" class="delta" :class="row.delta > 0 ? 'up' : 'down'">
                    {{ row.delta > 0 ? '+' : '' }}{{ formatNumber(row.delta) }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </section>
      </template>
    </template>
  </div>
</template>

<script setup>
// Two versions of one character side by side: identity, story, stats (typed
// and computed), gear and spells. Which two is kept in the URL (?a=&b=), so
// the page can be linked to and survives a reload.
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t, tr } from '../i18n'
import { fetchVersions, fetchVersion } from '../api'
import { EQUIP_SLOTS } from '../gear'
import BackLink from '../components/BackLink.vue'
import { statusLabel } from '../characterStatus'
import { pageTitle } from '../navigation'

const props = defineProps({ id: { type: String, required: true } })
const route = useRoute()
const router = useRouter()

const versions = ref([])
const loadError = ref('')
const loading = ref(false)
const a = ref(null)
const b = ref(null)
const onlyChanges = ref(true)

// Defaults: the version before the current one, against the current one.
const currentId = computed(() => String(versions.value.find((v) => v.is_current)?.id ?? versions.value[0]?.id ?? ''))
const otherId = computed(() => String(versions.value.find((v) => String(v.id) !== currentId.value)?.id ?? currentId.value))
const valid = (v) => versions.value.some((x) => String(x.id) === v)
const aId = computed(() => (valid(route.query.a) ? route.query.a : otherId.value))
const bId = computed(() => (valid(route.query.b) ? route.query.b : currentId.value))

function pick(which, value) {
  router.replace({ query: { ...route.query, a: aId.value, b: bId.value, [which]: value } })
}
function swap() {
  router.replace({ query: { ...route.query, a: bId.value, b: aId.value } })
}

const versionLabel = (v) =>
  [v.is_current ? `★ ${v.name}` : v.name, v.version_date, v.version_reference, t('Lv. {level}', { level: v.level })]
    .filter(Boolean)
    .join(' · ')

const show = (v) => (v === null || v === undefined || v === '' ? '—' : v)
const formatNumber = (n) => (Number.isInteger(n) ? String(n) : n.toFixed(2))
const round = (n, d = 0) => (typeof n === 'number' ? Math.round(n * 10 ** d) / 10 ** d : n)

function row(label, from, to, opts = {}) {
  const changed = String(from ?? '') !== String(to ?? '')
  const numeric = typeof from === 'number' && typeof to === 'number'
  return { label, from, to, changed, long: !!opts.long, delta: changed && numeric ? to - from : 0 }
}

const BASE = [
  ['hp', 'HP'], ['mp', 'MP'], ['attack', tr('Attack')], ['potency', tr('Potency')],
  ['defense', tr('Defense')], ['resistance', tr('Resistance')], ['speed', tr('Speed')],
]
const SPECIAL = [
  ['luck', tr('Luck'), 2], ['carry_limit', tr('Carry Limit'), 1], ['fall_damage_threshold', tr('Fall Damage Threshold'), 1],
  ['sanity', tr('Sanity'), 0], ['heat_threshold', tr('Heat Threshold'), 0], ['cold_threshold', tr('Cold Threshold'), 0],
]
const ELEMENTS = [
  ['resist_water', tr('Water')], ['resist_fire', tr('Fire')], ['resist_wind', tr('Wind')], ['resist_earth', tr('Earth')],
  ['resist_ice', tr('Ice')], ['resist_thunder', tr('Thunder')], ['resist_impact', tr('Impact')],
]
const LIFESKILLS = [
  ['cooking', tr('Cooking')], ['crafting', tr('Crafting')], ['alchemy', tr('Alchemy')], ['hunting', tr('Hunting')],
  ['gathering', tr('Gathering')], ['farming', tr('Farming')], ['blessing', tr('Blessing')],
  ['enchanting', tr('Enchanting')], ['smithing', tr('Smithing')],
]
const PRIMARY = [['vit', 'VIT'], ['def', 'DEF'], ['res', 'RES'], ['str', 'STR'], ['dex', 'DEX'], ['intel', 'INT'], ['wis', 'WIS'], ['agl', 'AGL']]

const groups = computed(() => {
  const x = a.value
  const y = b.value
  if (!x || !y) return []
  const s = (v) => v.story || {}
  const c = (v, part, key, d = 0) => round(v.computed?.[part]?.[key] ?? 0, d)
  const spellNames = (v) => (v.spells || []).map((sp) => sp.name).sort((p, q) => p.localeCompare(q))
  const xs = spellNames(x)
  const ys = spellNames(y)
  return [
    {
      title: t('Identity'),
      rows: [
        row(t('Name'), x.name, y.name),
        row(t('Nickname'), x.nickname, y.nickname),
        row(t('Level'), x.level, y.level),
        row(t('Class'), x.class_name, y.class_name),
        row(t('Subclass'), x.subclass_name, y.subclass_name),
        row(t('Specialization'), x.specialization_name, y.specialization_name),
        row(t('Version date'), x.version_date, y.version_date),
        row(t('Reference'), x.version_reference, y.version_reference),
      ],
    },
    {
      title: t('Story'),
      rows: [
        row(t('Gender'), s(x).gender, s(y).gender),
        row(t('Race'), s(x).race_name, s(y).race_name),
        row(t('Height (cm)'), s(x).height, s(y).height),
        row(t('Weight (kg)'), s(x).weight, s(y).weight),
        row(t('Body type'), s(x).body_type_name, s(y).body_type_name),
        row(t('Age'), s(x).age, s(y).age),
        row(t('Blood type'), s(x).blood_type, s(y).blood_type),
        row(t('Born in'), s(x).born_in_name || s(x).born_in, s(y).born_in_name || s(y).born_in),
        row(t('Nation'), s(x).nation_name || s(x).nation, s(y).nation_name || s(y).nation),
        row(t('Birth date (in-story)'), s(x).birth_date, s(y).birth_date),
        row(t('Human birth date'), s(x).human_birth_date, s(y).human_birth_date),
        row(t('Status'), statusLabel(s(x).status), statusLabel(s(y).status)),
        row(t('Deaths'), s(x).deaths, s(y).deaths),
        row(t('Description'), s(x).description, s(y).description, { long: true }),
        row(t('Bio'), s(x).bio, s(y).bio, { long: true }),
        row(t('Speech mannerisms'), s(x).speech_mannerisms, s(y).speech_mannerisms, { long: true }),
      ],
    },
    { title: t('Primary Stats'), rows: PRIMARY.map(([k, l]) => row(l, x.build?.[k] ?? 0, y.build?.[k] ?? 0)) },
    { title: t('Base Stats'), rows: BASE.map(([k, l]) => row(t(l), c(x, 'base', k), c(y, 'base', k))) },
    { title: t('Special Stats'), rows: SPECIAL.map(([k, l, d]) => row(t(l), c(x, 'special', k, d), c(y, 'special', k, d))) },
    // Multipliers (1.25x), so two decimals.
    { title: t('Special Defenses'), rows: ELEMENTS.map(([k, l]) => row(t(l), c(x, 'elemental', k, 2), c(y, 'elemental', k, 2))) },
    { title: t('Lifeskill'), rows: LIFESKILLS.map(([k, l]) => row(t(l), c(x, 'lifeskills', k), c(y, 'lifeskills', k))) },
    { title: t('Gear'), rows: EQUIP_SLOTS.map((sl) => row(t(sl.label), x.gear?.[sl.key]?.name, y.gear?.[sl.key]?.name)) },
    {
      title: t('Spells'),
      rows: [
        row(t('Learned'), '', ys.filter((n) => !xs.includes(n)).join(', ')),
        row(t('Forgotten'), xs.filter((n) => !ys.includes(n)).join(', '), ''),
        row(t('Total'), xs.length, ys.length),
      ],
    },
  ]
})

const changeCount = computed(() => groups.value.reduce((n, g) => n + g.rows.filter((r) => r.changed).length, 0))
const shownGroups = computed(() =>
  groups.value
    .map((g) => ({ ...g, rows: onlyChanges.value && aId.value !== bId.value ? g.rows.filter((r) => r.changed) : g.rows }))
    .filter((g) => g.rows.length),
)

async function loadPair() {
  if (!aId.value || !bId.value) return
  loading.value = true
  try {
    ;[a.value, b.value] = await Promise.all([fetchVersion(aId.value), fetchVersion(bId.value)])
    pageTitle.value = b.value.name
  } catch (err) {
    loadError.value = t("Couldn't load this version. Try refreshing.")
  } finally {
    loading.value = false
  }
}

watch([aId, bId], loadPair)

onMounted(async () => {
  try {
    versions.value = await fetchVersions(props.id)
  } catch (err) {
    loadError.value = t("Couldn't load versions. Try refreshing.")
    return
  }
  loadPair()
})
</script>

<style scoped>
.pickers {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  margin-bottom: 1rem;
}

.picker {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  flex: 1 1 240px;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.only-changes {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  margin-left: auto;
  min-height: var(--control-h);
}

.note {
  margin: 0 0 1rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.group {
  padding: 1rem 1.25rem;
  margin-bottom: 1rem;
}

.group h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.05rem;
  margin: 0 0 0.6rem;
}

.diff {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}

.diff th,
.diff td {
  padding: 0.4rem 0.5rem;
  border-top: 1px solid var(--glass-border);
  text-align: left;
  vertical-align: top;
}

.diff tr:first-child th,
.diff tr:first-child td {
  border-top: none;
}

.diff th {
  width: 26%;
  font-weight: 500;
  color: var(--text-muted);
}

.diff td.from,
.diff td.to {
  width: 35%;
  color: var(--text-faint);
}

.diff td.long {
  white-space: pre-wrap;
}

.diff td.arrow {
  width: 2rem;
  text-align: center;
  color: var(--accent);
}

.diff tr.is-changed td.to {
  color: var(--text-primary);
}

.diff tr.is-changed td.from {
  color: var(--text-muted);
}

.delta {
  margin-left: 0.35rem;
  font-size: 0.75rem;
  font-weight: 600;
}

.delta.up {
  color: var(--kind-founding);
}

.delta.down {
  color: var(--danger-text);
}

@media (max-width: 720px) {
  .diff th {
    width: 30%;
  }
  .only-changes {
    margin-left: 0;
  }
}
</style>
