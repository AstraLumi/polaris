// Asking "are you sure?" the app's way: one dialog (ConfirmDialog.vue, shown
// by App.vue) that any code can await, in place of the browser's own
// confirm() box, which looks like nothing else here, can't be translated or
// styled, and is easy to dismiss for good ("don't show again").
//
//   if (!(await askConfirm(t('Remove this picture?'), { confirmLabel: t('Remove') }))) return
import { ref } from 'vue'
import { t } from './i18n'

// The question on screen: { title, message, confirmLabel, danger, resolve }.
export const pendingConfirm = ref(null)

// Resolves true when confirmed, false when cancelled (Cancel, Esc, a click
// outside). With only a question, the question is the dialog's title.
export function askConfirm(question, { message = '', confirmLabel = '', danger = true } = {}) {
  // A second question while one is open answers the first with "no".
  pendingConfirm.value?.resolve(false)
  return new Promise((resolve) => {
    pendingConfirm.value = { title: question, message, confirmLabel, danger, resolve }
  })
}

export function answerConfirm(ok) {
  const q = pendingConfirm.value
  pendingConfirm.value = null
  q?.resolve(!!ok)
}

// The one asked most: leaving something typed and not saved.
export const confirmDiscard = () => askConfirm(t('Discard your changes?'), { message: t('What you typed here will be lost.'), confirmLabel: t('Discard') })
