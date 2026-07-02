import { ref } from 'vue'
import { useIntersectionObserver } from '@vueuse/core'

/**
 * SSR-safe "is this element scrolled into view" flag.
 *
 * @param {import('vue').Ref<HTMLElement | null>} targetRef - template ref to observe.
 * @param {{ threshold?: number, once?: boolean }} [options]
 * @param {number} [options.threshold=0.25] - IntersectionObserver threshold.
 * @param {boolean} [options.once=true] - stop observing after the first
 *   intersection so reveals don't replay when scrolling back.
 * @returns {{ inView: import('vue').Ref<boolean> }}
 */
export function useInView(targetRef, { threshold = 0.25, once = true } = {}) {
  const inView = ref(false)

  // SSR / no IntersectionObserver: leave inView=false. Consumers should treat
  // false as "not yet revealed"; reduced-motion / no-JS fallbacks are handled
  // by the timeline + CSS, not here.
  if (import.meta.env.SSR || typeof IntersectionObserver === 'undefined') {
    return { inView }
  }

  const { stop } = useIntersectionObserver(
    targetRef,
    (entries) => {
      const entry = entries[0]
      if (!entry) return
      if (entry.isIntersecting) {
        inView.value = true
        if (once) stop()
      } else if (!once) {
        inView.value = false
      }
    },
    { threshold }
  )

  return { inView }
}
