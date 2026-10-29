export type Stat = { label: string; value: string | number; pct?: number }

export function StatGrid({ items }: { items: Stat[] }) {
  return (
    <div className="cc-stats">
      {items.map((s) => (
        <div className="cc-stat" key={s.label}>
          <div className="cc-stat-label">{s.label}</div>
          <div className="cc-stat-box">
            <b className="cc-stat-value">{s.value}</b>
            <div
              className="cc-stat-ring"
              style={{ '--pct': s.pct === undefined ? 0 : Math.max(0, Math.min(100, s.pct)) } as any}
              aria-hidden="true"
            >
              <span />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
