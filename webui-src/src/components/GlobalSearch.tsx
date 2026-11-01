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
function pageTarget(node: Node): HTMLElement | null {
  const scope = (node.parentElement && node.parentElement.closest('.cc-main, .cc-sidebar')) as HTMLElement | null
  let el: HTMLElement | null = node.parentElement
  while (el && el !== scope) {
    if (
      el.matches(
        'button, [role="button"], a, .cc-trow, .cc-acctcard, .cc-sideitem, .cc-subitem, .cc-hrow, li, tr',
      )
    )
      return el
    el = el.parentElement
  }
  return (node.parentElement as HTMLElement) || null
}

function pageHits(needle: string): Hit[] {
  const out: Hit[] = []
  const seen = new Set<Element>()
  const roots = [document.querySelector('.cc-main'), document.querySelector('.cc-sidebar')].filter(
    Boolean,
  ) as HTMLElement[]
  for (const root of roots) {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
    let node: Node | null = walker.nextNode()
    while (node) {
      const text = (node.textContent || '').trim()
      if (text.length >= 2 && text.toLowerCase().includes(needle)) {
        const host = pageTarget(node)
        if (
          host &&
          host.offsetParent !== null &&
          !seen.has(host) &&
          !host.closest('.cc-gsearch, .cc-gsearch-sheet') &&
          !host.matches('.danger, .cc-rec-danger, [data-variant="destructive"]') &&
          !(host.getAttribute('aria-label') || '').match(/حذف|پاک/)
        ) {
          const label = (host.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 70)
          if (label.length >= 2) {
            seen.add(host)
            out.push({
              key: 'pg:' + out.length,
              label,
              sub: 'در این صفحه',
              run: () => {
                host.scrollIntoView({ block: 'center', behavior: 'smooth' })
                if (
                  host.tagName === 'BUTTON' ||
                  host.getAttribute('role') === 'button' ||
                  host.classList.contains('cc-trow') ||
                  host.classList.contains('cc-acctcard') ||
                  host.classList.contains('cc-hrow') ||
                  host.classList.contains('cc-sideitem')
                )
                  host.click()
              },
            })
            if (out.length >= 8) return out
          }
        }
      }
      node = walker.nextNode()
    }
  }
  return out
}

export function GlobalSearch({ go }: { go: (p: Provider | null, t: ToolId | null) => void }) {
  const [q, setQ] = useState('')
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
