import { Cloud, Globe, HardDrives, Moon, Sun, CaretDown, TelegramLogo } from '@phosphor-icons/react'
import { useState } from 'react'
import { useTheme } from './theme'
import type { ToolItem } from './Tools'

export type Provider = 'hz' | 'cf' | 'arv'

export type SubItem = { id: string; label: string; active?: boolean; onSelect: () => void }

export const PROVIDERS: { id: Provider; label: string; Icon: typeof Cloud }[] = [
  { id: 'hz', label: 'هتزنر', Icon: HardDrives },
  { id: 'cf', label: 'کلادفلر', Icon: Cloud },
  { id: 'arv', label: 'آروان', Icon: Globe },
]

export function Sidebar({
  provider,
  onChange,
  open,
  onToggle,
  sections,
  tools,
}: {
  provider: Provider
  onChange: (p: Provider) => void
  open: boolean
  onToggle: () => void
  sections?: SubItem[]
  tools?: ToolItem[]
}) {
  const { mode, toggle } = useTheme()
  const [expanded, setExpanded] = useState<Provider | null>(provider)

  return (
    <>
      <div className={'cc-scrim' + (open ? ' on' : '')} onClick={onToggle} aria-hidden="true" />
      <aside className={'cc-sidebar p-' + provider + (open ? ' open' : '')} aria-label="منوی اصلی">
        <div className="cc-sidebrand">
          <div className="cc-sidebrand-mark">CG</div>
          <div className="cc-sidebrand-text">
            <div className="cc-sidebrand-name">Cloud Guardian</div>
            <div className="cc-sidebrand-sub">کنسول ابری</div>
          </div>
          <button type="button" className="cc-drawer-x" aria-label="بستن منو" onClick={onToggle}>
            ✕
          </button>
        </div>

        <nav className="cc-sidenav" aria-label="سرویس‌ها">
          <div className="cc-sidenav-label">سرویس‌ها</div>
          {PROVIDERS.map((it) => {
            const on = provider === it.id
            const isOpen = expanded === it.id
            const hasSub = on && !!(sections && sections.length)
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

        {tools && tools.length > 0 && (
          <nav className="cc-sidenav" aria-label="ابزارها">
            <div className="cc-sidenav-label">ابزارها</div>
            {tools.map((t) => (
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

        <a className="cc-sideitem" href="https://t.me/openWEBcode" target="_blank" rel="noreferrer">
          <TelegramLogo size={19} />
          <span className="cc-sideitem-label">@openWEBcode</span>
        </a>

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
