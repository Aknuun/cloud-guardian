import { useState } from 'react'
import { MagnifyingGlass } from '@phosphor-icons/react'
import { TOOLS } from './toolsMeta'
import type { ToolId } from './toolsMeta'
import type { Provider } from './Sidebar'

const SERVICES: { id: Provider; label: string }[] = [
  { id: 'hz', label: 'هتزنر' },
  { id: 'cf', label: 'کلادفلر' },
  { id: 'arv', label: 'آروان کلاد' },
]

export function GlobalSearch({ go }: { go: (p: Provider | null, t: ToolId | null) => void }) {
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const items = [
    ...SERVICES.map((p) => ({ key: 'p:' + p.id, label: p.label, sub: 'سرویس', run: () => go(p.id, null) })),
    ...TOOLS.map((t) => ({ key: 't:' + t.id, label: t.label, sub: 'ابزار', run: () => go(null, t.id) })),
  ]
  const needle = q.trim().toLowerCase()
  const hits = needle ? items.filter((i) => i.label.toLowerCase().includes(needle)) : []
  const hitsBlock = hits.length > 0 && (
    <div className="cc-gres">
      {hits.map((h) => (
        <button
          key={h.key}
          type="button"
          onMouseDown={(e) => {
            e.preventDefault()
            h.run()
            setQ('')
            setOpen(false)
          }}
        >
          <span>{h.label}</span>
          <i>{h.sub}</i>
        </button>
      ))}
    </div>
  )
  return (
    <>
    <button
      type="button"
      className="cc-gsearch-btn"
      aria-label="جستجوی کلی"
      onClick={() => setOpen(true)}
    >
      <MagnifyingGlass size={18} />
    </button>
    {open && (
      <div className="cc-gsearch-sheet" onClick={() => { setOpen(false); setQ('') }}>
        <div className="cc-gsearch-sheetbox" onClick={(e) => e.stopPropagation()}>
          <input
            id="cc-gsearch-m"
            name="gsearchM"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="جستجوی کلی…"
            aria-label="جستجوی کلی"
          />
          {hitsBlock}
        </div>
      </div>
    )}
    <div className="cc-gsearch">
      <MagnifyingGlass size={16} />
      <input
        id="cc-gsearch"
        name="gsearch"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="جستجوی کلی…"
        aria-label="جستجوی کلی"
        onBlur={() => setTimeout(() => setQ(''), 150)}
      />
      {hitsBlock}
    </div>
    </>
  )
}
