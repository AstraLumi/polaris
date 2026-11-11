<template>
  <!-- "Deleted · Undo" after a deletion (undoDelete.js). -->
  <Transition name="undo-bar">
    <div v-if="pendingDelete" class="undo-bar glass-panel panel-solid" :class="{ 'is-error': failed }" :role="failed ? 'alert' : 'status'">
      <template v-if="failed">
        <span class="text">{{ pendingDelete.error || $t("Couldn't delete it.") }}</span>
        <button type="button" class="btn btn-ghost small" @click="dismissDeleteError">{{ $t('Close') }}</button>
      </template>
      <template v-else>
        <span class="text">{{ pendingDelete.label }}</span>
        <button type="button" class="btn btn-ghost small undo" @click="undoDelete">{{ $t('Undo') }}</button>
        <span :key="pendingDelete.label" class="time" :style="{ animationDuration: UNDO_MS + 'ms' }"></span>
      </template>
    </div>
  </Transition>
</template>

<script setup>
import { computed } from 'vue'
import { pendingDelete, undoDelete, dismissDeleteError, UNDO_MS } from '../undoDelete'

const failed = computed(() => !!pendingDelete.value && 'error' in pendingDelete.value)
</script>

<style scoped>
.undo-bar {
  position: fixed;
  left: 50%;
  bottom: calc(18px + env(safe-area-inset-bottom, 0px));
  transform: translateX(-50%);
  z-index: 900;
  display: flex;
  align-items: center;
  gap: 14px;
  max-width: min(520px, calc(100vw - 32px));
  padding: 10px 12px 10px 16px;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
}

.undo-bar.is-error {
  border-color: var(--danger);
}

.text {
  min-width: 0;
  overflow-wrap: anywhere;
}

.undo {
  flex-shrink: 0;
  color: var(--accent);
  font-weight: 600;
}

/* The time left to undo. */
.time {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 2px;
  width: 100%;
  background: var(--accent);
  transform-origin: left;
  animation: undo-time linear forwards;
}

@keyframes undo-time {
  to {
    transform: scaleX(0);
  }
}

.undo-bar-enter-active,
.undo-bar-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.undo-bar-enter-from,
.undo-bar-leave-to {
  opacity: 0;
  transform: translate(-50%, 8px);
}

@media (prefers-reduced-motion: reduce) {
  .undo-bar-enter-active,
  .undo-bar-leave-active {
    transition: none;
  }
}
</style>
