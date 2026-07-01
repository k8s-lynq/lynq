import { ref, onMounted, onScopeDispose } from 'vue'
import { useReducedMotion } from './useReducedMotion.js'

/**
 * Beat-driven step timeline for auto-playing walkthroughs.
 *
 * Drives a `step` index 0..steps-1 forward on a per-beat interval. The driver
 * is created only in onMounted (SSR-safe) and torn down on pause / scope
 * dispose. Manual scrubbing (seek/next/prev) pauses auto-advance.
 *
 * REDUCED-MOTION CONTRACT: when `respectReducedMotion` is true and the user
 * prefers reduced motion, the timeline never auto-advances — it immediately
 * seeks to the final settled frame (steps-1) and leaves manual controls usable.
 *
 * @param {object} [opts]
 * @param {number} [opts.steps=1] - number of beats (indices 0..steps-1).
 * @param {number[] | number} [opts.durations] - per-beat duration in ms
 *   (array indexed by step) or a single number applied to every beat.
 *   Falls back to ~1400ms.
 * @param {boolean} [opts.loop=false] - wrap from last beat back to 0.
 * @param {boolean} [opts.autoStart=false] - begin playing on mount.
 * @param {boolean} [opts.respectReducedMotion=true]
 * @returns {{
 *   step: import('vue').Ref<number>,
 *   playing: import('vue').Ref<boolean>,
 *   progress: import('vue').Ref<number>,
 *   play: () => void,
 *   pause: () => void,
 *   toggle: () => void,
 *   seek: (i: number) => void,
 *   next: () => void,
 *   prev: () => void,
 * }}
 */
export function useStepTimeline({
  steps = 1,
  durations,
  loop = false,
  autoStart = false,
  respectReducedMotion = true,
} = {}) {
  const total = Math.max(1, steps)
  const step = ref(0)
  const playing = ref(false)
  const progress = ref(0)

  const reduced = respectReducedMotion ? useReducedMotion() : ref(false)

  const DEFAULT_BEAT = 1400
  function beatDuration(i) {
    if (Array.isArray(durations)) {
      return durations[i] ?? durations[durations.length - 1] ?? DEFAULT_BEAT
    }
    if (typeof durations === 'number') return durations
    return DEFAULT_BEAT
  }

  let timer = null
  let mounted = false

  function clearTimer() {
    if (timer !== null) {
      clearInterval(timer)
      timer = null
    }
  }

  function clamp(i) {
    if (i < 0) return 0
    if (i > total - 1) return total - 1
    return i
  }

  function scheduleNextBeat() {
    clearTimer()
    if (!playing.value) return
    const delay = beatDuration(step.value)
    progress.value = 0
    timer = setInterval(() => {
      if (step.value >= total - 1) {
        if (loop) {
          step.value = 0
        } else {
          pause()
          return
        }
      } else {
        step.value = step.value + 1
      }
      // Re-schedule with the (possibly different) duration of the new beat.
      scheduleNextBeat()
    }, delay)
  }

  function play() {
    // Never auto-advance under reduced motion; show the settled final frame.
    if (reduced.value) {
      playing.value = false
      step.value = total - 1
      progress.value = 1
      clearTimer()
      return
    }
    if (!mounted) {
      // Defer until onMounted wires the driver; remember intent via autoStart.
      playing.value = true
      return
    }
    playing.value = true
    scheduleNextBeat()
  }

  function pause() {
    playing.value = false
    clearTimer()
  }

  function toggle() {
    if (playing.value) pause()
    else play()
  }

  // seek/next/prev are scrub actions: they pause auto-advance.
  function seek(i) {
    pause()
    step.value = clamp(i)
    progress.value = 0
  }

  function next() {
    pause()
    step.value = loop && step.value >= total - 1 ? 0 : clamp(step.value + 1)
    progress.value = 0
  }

  function prev() {
    pause()
    step.value = loop && step.value <= 0 ? total - 1 : clamp(step.value - 1)
    progress.value = 0
  }

  onMounted(() => {
    mounted = true
    if (reduced.value) {
      // Settle on the final frame; manual controls remain usable.
      step.value = total - 1
      progress.value = 1
      return
    }
    if (autoStart || playing.value) {
      play()
    }
  })

  onScopeDispose(clearTimer)

  return { step, playing, progress, play, pause, toggle, seek, next, prev }
}
