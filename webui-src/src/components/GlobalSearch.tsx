import { useMemo, useState } from 'react'
import { MagnifyingGlass } from '@phosphor-icons/react'
import { toolsVisible } from './toolsMeta'
import type { ToolId } from './toolsMeta'
import type { Provider } from './Sidebar'

const SERVICES: { id: Provider; label: string }[] = [
  { id: 'hz', label: 'هتزنر' },
  { id: 'cf', label: 'کلادفلر' },
  { id: 'arv', label: 'آروان کلاد' },
]

type Hit = { key: string; label: string; sub: string; run: () => void }

// Everything the user can see on the current page: card titles, section heads,
// submenus and plain buttons (destructive ones are skipped).
function pageHits(needle: string): Hit[] {
  const out: Hit[] = []
  const seen = new Set<string>()
  const nodes = document.querySelectorAll<HTMLElement>(
    '.cc-main button, .cc-main .cc-name, .cc-main .cc-acctcard, .cc-main .cc-section-head h2, ' +
      '.cc-main .cc-hg-title, .cc-main .cc-trow, .cc-sidebar .cc-sideitem, .cc-sidebar .cc-subitem, ' +
      '.cc-sidebar .cc-sidenav-label',
  )
  nodes.forEach((el) => {
    if (el.offsetParent === null) return
    if (el.closest('.cc-gsearch') || el.closest('.cc-gsearch-sheet')) return
    if (el.classList.contains('danger') || el.classList.contains('cc-rec-danger')) return
    const aria = el.getAttribute('aria-label') || ''
    if (aria.includes('حذف') || aria.includes('پاک')) return
    const label = (el.textContent || el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 70)
    if (label.length < 2 || !label.toLowerCase().includes(needle)) return
    const key = label.toLowerCase() + '|' + el.tagName + '|' + el.className
    if (seen.has(key)) return
    seen.add(key)
    out.push({
      key,
      label,
      sub: 'در این صفحه',
      run: () => {
        el.scrollIntoView({ block: 'center', behavior: 'smooth' })
        if (el.tagName === 'BUTTON' || el.getAttribute('role') === 'button') el.click()
      },
    })
  })
  return out.slice(0, 8)
}

export function GlobalSearch({ go }: { go: (p: Provider | null, t: ToolId | null) => void }) {
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const needle = q.trim().toLowerCase()

  const hits = useMemo(() => {
    if (needle.length < 2) return [] as Hit[]
    const nav: Hit[] = [
      ...SERVICES.map((p) => ({ key: 'p:' + p.id, label: p.label, sub: 'سرویس', run: () => go(p.id, null) })),
      ...toolsVisible().map((t) => ({ key: 't:' + t.id, label: t.label, sub: 'ابزار', run: () => go(null, t.id) })),
    ].filter((i) => i.label.toLowerCase().includes(needle))
    return [...nav, ...pageHits(needle)]
  }, [needle, go])

  function pick(h: Hit) {
    h.run()
    setQ('')
    setOpen(false)
  }

  const list = (
    <div className="cc-gres" role="listbox" aria-label="نتایج جستجو">
      {hits.length === 0 && <div className="cc-gres-empty">چیزی پیدا نشد</div>}
      {hits.map((h) => (
        <button key={h.key} type="button" role="option" onMouseDown={(e) => { e.preventDefault(); pick(h) }}>
          <span className="cc-gres-label">{h.label}</span>
          <i>{h.sub}</i>
        </button>
      ))}
    </div>
  )

  return (
    <>
      <button type="button" className="cc-gsearch-btn" aria-label="جستجوی کلی" onClick={() => setOpen(true)}>
        <MagnifyingGlass size={18} />
      </button>

      {open && (
        <div className="cc-gsearch-sheet" onClick={() => { setOpen(false); setQ('') }}>
          <div className="cc-gsearch-sheetbox" onClick={(e) => e.stopPropagation()}>
            <div className="cc-gsearch-field">
              <MagnifyingGlass size={16} />
              <input
                id="cc-gsearch-m"
                name="gsearchM"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="جستجوی صفحه، کارت‌ها و دکمه‌ها…"
                aria-label="جستجوی کلی"
              />
            </div>
            {needle.length >= 2 && list}
          </div>
        </div>
      )}

      <div className="cc-gsearch">
        <MagnifyingGlass size={16} className="cc-gsearch-ico" />
        <input
          id="cc-gsearch"
          name="gsearch"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="جستجو در صفحه، کارت‌ها و دکمه‌ها…"
          aria-label="جستجوی کلی"
          onBlur={() => setTimeout(() => setQ(''), 180)}
        />
        {needle.length >= 2 && list}
      </div>
    </>
  )
}
