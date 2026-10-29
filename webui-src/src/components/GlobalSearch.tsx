import { useState } from 'react'
import { MagnifyingGlass } from '@phosphor-icons/react'
import { TOOLS } from './Tools'
import type { ToolId } from './Tools'
import type { Provider } from './Sidebar'

const SERVICES: { id: Provider; label: string }[] = [
  { id: 'hz', label: 'هتزنر' },
  { id: 'cf', label: 'کلادفلر' },
  { id: 'arv', label: 'آروان' },
]

export function GlobalSearch({ go }: { go: (p: Provider | null, t: ToolId | null) => void }) {
  const [q, setQ] = useState('')
  const items = [
    ...SERVICES.map((p) => ({ key: 'p:' + p.id, label: p.label, sub: 'سرویس', run: () => go(p.id, null) })),
    ...TOOLS.map((t) => ({ key: 't:' + t.id, label: t.label, sub: 'ابزار', run: () => go(null, t.id) })),
  ]
  const needle = q.trim().toLowerCase()
  const hits = needle ? items.filter((i) => i.label.toLowerCase().includes(needle)) : []
  return (
    <div className="cc-gsearch">
      <MagnifyingGlass size={16} />
      <input
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="جستجوی کلی…"
        aria-label="جستجوی کلی"
        onBlur={() => setTimeout(() => setQ(''), 150)}
      />
      {hits.length > 0 && (
        <div className="cc-gres">
          {hits.map((h) => (
            <button
              key={h.key}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault()
                h.run()
                setQ('')
              }}
            >
              <span>{h.label}</span>
              <i>{h.sub}</i>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
