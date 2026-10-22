import { cn } from './utils'

export function Meter({
  label,
  value,
  max = 100,
  showValue = true,
  className,
}: {
  label?: string
  value: number
  max?: number
  showValue?: boolean
  className?: string
}) {
  const pct = Math.max(0, Math.min(100, max > 0 ? (value / max) * 100 : 0))
  const tone = pct >= 90 ? 'bg-[var(--ds-danger)]' : pct >= 70 ? 'bg-[var(--ds-warning)]' : 'bg-primary'
  return (
    <div className={cn('w-full', className)}>
      {(label || showValue) && (
        <div className="mb-1.5 flex items-center justify-between gap-2 text-xs">
          {label ? <span className="text-muted-foreground">{label}</span> : <span />}
          {showValue ? <span className="font-medium tabular-nums text-foreground">{Math.round(pct)}%</span> : null}
        </div>
      )}
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
        <div className={cn('h-full rounded-full transition-[width] duration-500', tone)} style={{ width: pct + '%' }} />
      </div>
    </div>
  )
}
