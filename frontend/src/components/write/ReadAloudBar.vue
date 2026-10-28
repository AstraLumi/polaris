<template>
  <div class="read-bar glass-panel panel-solid" role="group" :aria-label="$t('Read aloud')">
    <button type="button" class="rb" :title="$t('Previous sentence')" :aria-label="$t('Previous sentence')" @click="$emit('step', -1)">⏮</button>
    <button type="button" class="rb main" :title="playing ? $t('Pause') : $t('Resume')" :aria-label="playing ? $t('Pause') : $t('Resume')" @click="$emit('toggle')">{{ playing ? '⏸' : '▶' }}</button>
    <button type="button" class="rb" :title="$t('Next sentence')" :aria-label="$t('Next sentence')" @click="$emit('step', 1)">⏭</button>
    <button type="button" class="rb" :title="$t('Stop reading') + ' (Esc)'" :aria-label="$t('Stop reading')" @click="$emit('stop')">⏹</button>
    <span class="count">{{ info.index + 1 }} / {{ info.total }}</span>
    <label class="rate" :title="$t('Speed')">
      <span class="sr-only">{{ $t('Speed') }}</span>
      <input type="range" min="0.5" max="2" step="0.1" :value="info.rate" :aria-label="$t('Speed')" @input="$emit('rate', +$event.target.value)" />
      <span class="rate-val">{{ info.rate.toFixed(1) }}×</span>
    </label>
    <select v-if="hasVoices" class="voice" :value="selected" :aria-label="$t('Voice')" :title="$t('Voice')" @change="$emit('voice', $event.target.value)">
      <optgroup v-if="groups.matching.length" :label="$t('Voices for this book’s language')">
        <option v-for="v in groups.matching" :key="v.voiceURI" :value="v.voiceURI">{{ v.name }}</option>
      </optgroup>
      <optgroup v-if="groups.others.length" :label="$t('Other voices')">
        <option v-for="v in groups.others" :key="v.voiceURI" :value="v.voiceURI">{{ v.name }}</option>
      </optgroup>
    </select>
  </div>
</template>

<script setup>
// The floating control while the chapter is read aloud (src/editor/readAloudPlugin.js).
import { computed } from 'vue'
import { groupVoices, pickVoice } from '../../editor/readAloud.js'

const props = defineProps({
  info: { type: Object, required: true }, // { status, index, total, rate, voice, voices }
  language: { type: String, default: 'en' },
})
defineEmits(['toggle', 'stop', 'step', 'rate', 'voice'])

const playing = computed(() => props.info.status === 'playing')
const groups = computed(() => groupVoices(props.info.voices, props.language))
const hasVoices = computed(() => groups.value.matching.length + groups.value.others.length > 0)
const selected = computed(() => pickVoice(props.info.voices, props.info.voice, props.language)?.voiceURI || '')
</script>

<style scoped>
.read-bar {
  position: fixed;
  left: 50%;
  bottom: 1rem;
  transform: translateX(-50%);
  z-index: 20;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.3rem 0.5rem;
  max-width: calc(100vw - 2rem);
  padding: 0.4rem 0.7rem;
  border-radius: 12px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.3);
}

.rb {
  min-width: 2.2rem;
  height: 2.2rem;
  border: 1px solid transparent;
  border-radius: 8px;
  background: none;
  color: var(--text-primary);
  font-size: 1rem;
  cursor: pointer;
}

.rb:hover {
  background: var(--glass);
  border-color: var(--glass-border);
}

.rb.main {
  background: var(--accent-soft);
}

.count {
  font-size: 0.78rem;
  color: var(--text-faint);
  min-width: 3.2rem;
  text-align: center;
}

.rate {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.rate input {
  width: 6rem;
}

.rate-val {
  font-size: 0.78rem;
  min-width: 2.2rem;
  color: var(--text-secondary, var(--text-faint));
}

.voice {
  max-width: 11rem;
  min-height: 2.1rem;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
</style>
