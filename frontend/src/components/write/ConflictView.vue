<template>
  <div class="overlay conflict-overlay">
    <div class="modal glass-panel panel-solid conflict" role="dialog" aria-modal="true" :aria-label="title">
      <h2>{{ title }}</h2>
      <p class="lead">{{ message }}</p>
      <div class="columns">
        <section class="col">
          <header class="col-head">
            <strong>{{ mineLabel }}</strong>
            <span class="meta">{{ $tn('{n} word', '{n} words', countDocJSON(mine)) }}<template v-if="mineWhen"> · {{ mineWhen }}</template></span>
          </header>
          <div class="paper"><DocPreview :doc="mine" :online="online" /></div>
          <button type="button" class="btn btn-primary" :disabled="busy" @click="$emit('keep', 'mine')">{{ keepMineLabel }}</button>
        </section>
        <section class="col">
          <header class="col-head">
            <strong>{{ theirsLabel }}</strong>
            <span class="meta">{{ $tn('{n} word', '{n} words', countDocJSON(theirs)) }}<template v-if="theirsWhen"> · {{ theirsWhen }}</template></span>
          </header>
          <div class="paper"><DocPreview :doc="theirs" :online="online" /></div>
          <button type="button" class="btn btn-ghost" :disabled="busy" @click="$emit('keep', 'theirs')">{{ keepTheirsLabel }}</button>
        </section>
      </div>
      <p class="foot">{{ $t('The text you don\'t keep is saved as a version, so nothing is lost.') }}</p>
      <div v-if="cancellable" class="modal-actions">
        <button type="button" class="btn btn-ghost" :disabled="busy" @click="$emit('cancel')">{{ $t('Decide later') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
// Two texts of one chapter side by side, and a button under each to keep
// it: after a save found the chapter changed elsewhere (409), or when this
// browser has a copy the server doesn't.
import { countDocJSON } from '../../editor/wordCount.js'
import DocPreview from './DocPreview.vue'

defineProps({
  title: { type: String, required: true },
  message: { type: String, default: '' },
  mine: { type: Object, required: true },
  theirs: { type: Object, required: true },
  mineLabel: { type: String, required: true },
  theirsLabel: { type: String, required: true },
  keepMineLabel: { type: String, required: true },
  keepTheirsLabel: { type: String, required: true },
  mineWhen: { type: String, default: '' },
  theirsWhen: { type: String, default: '' },
  online: { type: Boolean, default: false },
  busy: { type: Boolean, default: false },
  cancellable: { type: Boolean, default: false },
})
defineEmits(['keep', 'cancel'])
</script>

<style scoped>
.conflict-overlay {
  z-index: 80;
}

.conflict {
  max-width: 1100px;
  display: flex;
  flex-direction: column;
}

.lead {
  margin: -0.6rem 0 1rem;
  color: var(--text-muted);
  font-size: 0.9rem;
  line-height: 1.5;
}

.columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
  min-height: 0;
}

.col {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 0;
}

.col-head {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.meta {
  font-size: 0.78rem;
  color: var(--text-faint);
}

.paper {
  height: min(52vh, 34rem);
  overflow: auto;
  padding: 1rem 1.2rem;
  border-radius: 10px;
  background: var(--paper, #fbf8f1);
  color: var(--ink, #1f1c17);
}

.foot {
  margin: 0.9rem 0 0;
  font-size: 0.8rem;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  .columns {
    grid-template-columns: 1fr;
  }
  .paper {
    height: 30vh;
  }
}
</style>
