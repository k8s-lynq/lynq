import { useEffect, useRef, useState } from 'react'
import { ANIM_TOOLTIP_DELAY } from '../constants'

interface TooltipProps {
  x: number
  y: number
  lines: string[]
  visible: boolean
}

export function FloatingTooltip({ x, y, lines, visible }: TooltipProps) {
  const [show, setShow] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current)
    if (visible) {
      timerRef.current = setTimeout(() => setShow(true), ANIM_TOOLTIP_DELAY)
    } else {
      setShow(false)
    }
    return () => { if (timerRef.current) clearTimeout(timerRef.current) }
  }, [visible])

  if (!show || !visible) return null

  return (
    <div
      className="pointer-events-none absolute z-50 rounded-md border bg-popover px-3 py-2 text-sm shadow-md max-w-[220px]"
      style={{ left: x + 12, top: y - 8 }}
    >
      {lines.map((l, i) => (
        <p key={i} className={i === 0 ? 'font-medium text-foreground' : 'text-muted-foreground text-xs mt-0.5'}>
          {l}
        </p>
      ))}
    </div>
  )
}
