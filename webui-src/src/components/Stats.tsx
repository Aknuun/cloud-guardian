export type Stat = { label: string; value: string | number; pct?: number }

export function StatGrid({ items }: { items: Stat[] }) {
  return (
    <div className="cc-stats">
      {items.map((s) => (
        <div className="cc-stat" key={s.label}>
          <div className="cc-stat-label">{s.label}</div>
          <div className="cc-stat-box">
            <b className="cc-stat-value">{s.value}</b>
            {s.pct !== undefined && s.pct > 0 && (
              <div
                className="cc-stat-ring"
                style={{ '--pct': Math.max(0, Math.min(100, s.pct)) } as any}
                aria-hidden="true"
              >
                <span />
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
