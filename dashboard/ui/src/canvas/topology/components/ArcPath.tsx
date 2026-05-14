import { arc as d3Arc } from 'd3-shape'
import { useMemo } from 'react'

interface ArcPathProps extends React.SVGProps<SVGPathElement> {
  startAngle: number
  endAngle: number
  innerRadius: number
  outerRadius: number
}

export function ArcPath({ startAngle, endAngle, innerRadius, outerRadius, ...props }: ArcPathProps) {
  const d = useMemo(() => {
    const gen = d3Arc<null>()
      .innerRadius(innerRadius)
      .outerRadius(outerRadius)
      .startAngle(startAngle)
      .endAngle(endAngle)
      .padAngle(0.015)
      .padRadius(80)
      .cornerRadius(3)
    return gen(null) ?? ''
  }, [startAngle, endAngle, innerRadius, outerRadius])

  return <path d={d} {...props} />
}
