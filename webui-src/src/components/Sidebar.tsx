import { Cloud, Globe, HardDrives, Moon, Sun, CaretDown } from '@phosphor-icons/react'
import { useEffect, useRef, useState } from 'react'
import { useTheme } from './theme'
import { useVersion } from './AppVersion'
import { BrandMark } from './BrandMark'
import type { ToolItem } from './toolsMeta'

export type Provider = 'hz' | 'cf' | 'arv'

export type SubItem = { id: string; label: string; active?: boolean; onSelect: () => void }

const PG_IDS = ['hs', 'pg', 'hook', 'hf', 'hfd', 'um']
const MISC_IDS = ['hub', 'fo', 'rem', 'set']

export const PROVIDERS: { id: Provider; label: string; Icon: typeof Cloud }[] = [
  { id: 'cf', label: 'کلادفلر', Icon: Cloud },
  { id: 'hz', label: 'هتزنر', Icon: HardDrives },
  { id: 'arv', label: 'آروان کلاد', Icon: Globe },
]

export function Sidebar({
  provider,
  onChange,
  open,
  onToggle,
  sections,
  tools,
  providerActive = true,
}: {
  provider: Provider
  onChange: (p: Provider) => void
  open: boolean
  onToggle: () => void
  sections?: SubItem[]
  tools?: ToolItem[]
  providerActive?: boolean
}) {
  const { mode, toggle } = useTheme()
  const [expanded, setExpanded] = useState<Provider | null>(provider)
  const home = tools?.find((t) => t.id === 'home')

  // swipe from the right edge opens the drawer, swiping back closes it
  useEffect(() => {
    let x0 = 0
    let y0 = 0
    let track = false
    const start = (e: TouchEvent) => {
      const t = e.touches[0]
      if (!t) return
      track = open || t.clientX > window.innerWidth - 30
      x0 = t.clientX
      y0 = t.clientY
    }
    const end = (e: TouchEvent) => {
      const t = e.changedTouches[0]
      if (!track || !t) return
      track = false
      const dx = t.clientX - x0
      if (Math.abs(t.clientY - y0) > 70 || Math.abs(dx) < 55) return
      if (!open && dx < 0) onToggle()
      else if (open && dx > 0) onToggle()
    }
    window.addEventListener('touchstart', start, { passive: true })
    window.addEventListener('touchend', end, { passive: true })
    return () => {
      window.removeEventListener('touchstart', start)
      window.removeEventListener('touchend', end)
    }
  }, [open, onToggle])
  const asideRef = useRef<HTMLElement | null>(null)
  const [sbw, setSbw] = useState(0)
  // fit the rail to its longest label instead of a magic width
  useEffect(() => {
    const el = asideRef.current
    if (!el || !open) return
    const labels = Array.from(el.querySelectorAll<HTMLElement>('.cc-sideitem-label'))
    const widest = labels.reduce((m, l) => Math.max(m, l.scrollWidth), 0)
    if (widest > 0) setSbw(Math.min(320, Math.ceil(widest) + 62))
  }, [open, tools, provider, expanded])

  const version = useVersion()
  const rest = tools?.filter((t) => t.id !== 'home' && !PG_IDS.includes(t.id) && !MISC_IDS.includes(t.id))
  const misc = tools?.filter((t) => MISC_IDS.includes(t.id))
  const pgItems = tools?.filter((t) => PG_IDS.includes(t.id))

  return (
    <>
      <div className={'cc-scrim' + (open ? ' on' : '')} onClick={onToggle} aria-hidden="true" />
      <aside
        ref={asideRef}
        className={'cc-sidebar p-' + provider + (open ? ' open' : '')}
        style={sbw ? ({ '--sb-w': sbw + 'px' } as React.CSSProperties) : undefined}
        aria-label="منوی اصلی"
      >
        <div className="cc-sidebrand">
          <div className="cc-sidebrand-mark"><BrandMark size={40} /></div>
          <div className="cc-sidebrand-text">
            <div className="cc-sidebrand-name">Cloud Guardian</div>
            <div className="cc-sidebrand-sub" dir="ltr">{version ? 'v' + version : 'Cloud Guardian'}</div>
          </div>
          <button type="button" className="cc-drawer-x" aria-label="بستن منو" onClick={onToggle}>
            ✕
          </button>
        </div>

        {home && (
          <nav className="cc-sidenav cc-navhome" aria-label="خانه">
            <button
              type="button"
              className={'cc-sideitem' + (home.active ? ' on' : '')}
              aria-current={home.active ? 'page' : undefined}
              onClick={() => {
                home.onSelect()
                if (window.innerWidth < 900) onToggle()
              }}
            >
              <home.Icon size={19} weight={home.active ? 'fill' : 'regular'} />
              <span className="cc-sideitem-label">{home.label}</span>
            </button>
          </nav>
        )}

        <nav className="cc-sidenav" aria-label="سرویس‌ها">
          <div className="cc-sidenav-label">سرویس‌ها</div>
          {PROVIDERS.map((it) => {
            const on = providerActive && provider === it.id
            const isOpen = expanded === it.id
            const hasSub = on && !!(sections?.length)
            const items = hasSub && isOpen ? sections || [] : []
            return (
              <div key={it.id} className="cc-navgroup">
                <button
                  type="button"
                  className={'cc-sideitem' + (on ? ' on' : '')}
                  aria-current={on ? 'page' : undefined}
                  aria-expanded={isOpen}
                  onClick={() => {
                    if (!on) onChange(it.id)
                    if (!open) {
                      setExpanded(it.id)
                      onToggle()
                      return
                    }
                    setExpanded(isOpen ? null : it.id)
                  }}
                >
                  <it.Icon size={19} weight={on ? 'fill' : 'regular'} />
                  <span className="cc-sideitem-label">{it.label}</span>
                  {hasSub && (
                    <CaretDown size={14} className={'cc-caret' + (isOpen ? ' open' : '')} aria-hidden="true" />
                  )}
                </button>
                {items.length > 0 && (
                  <div className="cc-subnav">
                    {items.map((s) => (
                      <button
                        key={s.id}
                        type="button"
                        className={'cc-subitem' + (s.active ? ' on' : '')}
                        onClick={() => {
                          s.onSelect()
                          if (window.innerWidth < 900) onToggle()
                        }}
                      >
                        {s.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )
          })}
        </nav>

        {pgItems && pgItems.length > 0 && (
          <nav className="cc-sidenav" aria-label="پاسارگارد">
            <div className="cc-sidenav-label">پاسارگارد</div>
            {pgItems.map((t) => (
              <button
                key={t.id}
                type="button"
                className={'cc-sideitem' + (t.active ? ' on' : '')}
                aria-current={t.active ? 'page' : undefined}
                onClick={() => {
                  t.onSelect()
                  if (window.innerWidth < 900) onToggle()
                }}
              >
                <t.Icon size={19} weight={t.active ? 'fill' : 'regular'} />
                <span className="cc-sideitem-label">{t.label}</span>
              </button>
            ))}
          </nav>
        )}

        {rest && rest.length > 0 && (
          <nav className="cc-sidenav" aria-label="سرور و دامنه">
            <div className="cc-sidenav-label">سرور و دامنه</div>
            {rest.map((t) => (
              <button
                key={t.id}
                type="button"
                className={'cc-sideitem' + (t.active ? ' on' : '')}
                aria-current={t.active ? 'page' : undefined}
                onClick={() => {
                  t.onSelect()
                  if (window.innerWidth < 900) onToggle()
                }}
              >
                <t.Icon size={19} weight={t.active ? 'fill' : 'regular'} />
                <span className="cc-sideitem-label">{t.label}</span>
              </button>
            ))}
          </nav>
        )}

        {misc && misc.length > 0 && (
          <nav className="cc-sidenav" aria-label="ابزارها">
            <div className="cc-sidenav-label">ابزارها</div>
            {misc.map((t) => (
              <button
                key={t.id}
                type="button"
                className={'cc-sideitem' + (t.active ? ' on' : '')}
                aria-current={t.active ? 'page' : undefined}
                onClick={() => {
                  t.onSelect()
                  if (window.innerWidth < 900) onToggle()
                }}
              >
                <t.Icon size={19} weight={t.active ? 'fill' : 'regular'} />
                <span className="cc-sideitem-label">{t.label}</span>
              </button>
            ))}
          </nav>
        )}

        <nav className="cc-sidenav" aria-label="تنظیمات">
          <div className="cc-sidenav-label">تنظیمات</div>
          <button type="button" className="cc-sideitem" onClick={toggle}>
            {mode === 'dark' ? <Sun size={19} /> : <Moon size={19} />}
            <span className="cc-sideitem-label">{mode === 'dark' ? 'حالت روشن' : 'حالت تاریک'}</span>
          </button>
        </nav>

        <div className="cc-profile">
          <div className="cc-profile-avatar">م</div>
          <div className="cc-grow">
            <div className="cc-profile-name">مدیر</div>
            <div className="cc-profile-role">دسترسی کامل</div>
          </div>
        </div>
      </aside>
    </>
  )
}

export function ThemeToggle() {
  const { mode, toggle } = useTheme()
  return (
    <button type="button" className="cc-themebtn" aria-label="تغییر تم" onClick={toggle}>
      {mode === 'dark' ? <Sun size={19} /> : <Moon size={19} />}
    </button>
  )
}
