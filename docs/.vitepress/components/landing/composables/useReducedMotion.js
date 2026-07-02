import { computed, ref } from 'vue'
import { usePreferredReducedMotion } from '@vueuse/core'

/**
 * SSR-safe reduced-motion preference.
 *
 * Wraps @vueuse/core's `usePreferredReducedMotion` (a string ref:
 * 'reduce' | 'no-preference' | ...) and maps it to a boolean. @vueuse guards
 * `window` / `matchMedia` internally; on the client it becomes reactive after
 * hydration.
 *
 * @returns {import('vue').Ref<boolean>} reactive `true` when the user has
 *   requested reduced motion.
 */
export function useReducedMotion() {
  // During SSR there is no matchMedia; default to false (motion allowed as the
  // static/settled frame). @vueuse handles the guard, but be defensive so this
  // never throws at module-eval or setup() time under Node.
  if (import.meta.env.SSR || typeof window === 'undefined') {
    return ref(false)
  }
  const pref = usePreferredReducedMotion()
  return computed(() => pref.value === 'reduce')
}
