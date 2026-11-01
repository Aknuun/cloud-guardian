import { faNum } from '../lib'

// A tile is only a "ratio" when we actually know its denominator. Otherwise it is
// a plain number (compact card) and a lone zero shows an em dash instead of "0".
export type Stat = { label: string; value: string | number; of?: number; pct?: number; note?: string }

function isZero(v: string | number) {
  if (typeof v === 'number') return v === 0
  return v.trim() === '0' || v.trim() === '۰'
}

export function StatGrid({ items }: { items: Stat[] }) {
  return (
    <div className="cc-stats">
      {items.map((s) => {
        const num = typeof s.value === 'number' ? s.value : Number(String(s.value).replace(/[۰-۹]/g, (d) => String('۰۱۲۳۴۵۶۷۸۹'.indexOf(d))))
        const ratio = s.of !== undefined ? (s.of > 0 && Number.isFinite(num) ? Math.round((num / s.of) * 100) : 0) : s.pct
        const showRing = typeof ratio === 'number' && ratio > 0 && ratio <= 100
        const zero = isZero(s.value)
        const compact = !showRing && s.of === undefined
        const text = zero && s.of === undefined ? '—' : typeof s.value === 'number' ? faNum(s.value) : s.value
        return (
          <div className={'cc-stat' + (compact ? ' compact' : '')} key={s.label}>
            <div className="cc-stat-label">{s.label}</div>
            <div className="cc-stat-box">
              <b className="cc-stat-value">{text}</b>
              {s.of !== undefined && <span className="cc-stat-of">از {faNum(s.of)}</span>}
              {showRing ? (
                <div
                  className="cc-stat-ring"
                  style={{ '--pct': Math.max(0, Math.min(100, ratio)) } as unknown as React.CSSProperties}
                  aria-hidden="true"
                >
                  <span />
                </div>
              ) : s.note ? (
                <span className="cc-stat-note">{s.note}</span>
              ) : null}
            </div>
          </div>
        )
      })}
    </div>
  )
}
