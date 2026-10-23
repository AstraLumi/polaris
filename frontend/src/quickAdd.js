// "Add" links such as /characters?new=1 (the search palette's actions): the
// list page opens its own add form, and the query is cleared once it has, so
// reloading the page or coming back to it doesn't open the form again.

import { watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

export function useQuickAdd(open) {
  const route = useRoute()
  const router = useRouter()
  watch(
    () => route.query.new,
    async (flag) => {
      if (flag === undefined) return
      const { new: _gone, ...rest } = route.query
      await router.replace({ path: route.path, query: rest, hash: route.hash })
      open()
    },
    { immediate: true },
  )
}
