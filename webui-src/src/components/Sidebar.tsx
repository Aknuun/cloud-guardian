import { Cloud, Cube, Globe, HardDrives, Moon, Sun, X } from '@phosphor-icons/react'
import { useTheme } from './theme'

export type Provider = 'hz' | 'cf' | 'ln' | 'arv'

export const PROVIDERS: { id: Provider; label: string; Icon: typeof Cloud }[] = [
  { id: 'hz', label: 'هتزنر', Icon: HardDrives },
  { id: 'cf', label: 'کلادفلر', Icon: Cloud },
  { id: 'ln', label: 'لینود', Icon: Cube },
  { id: 'arv', label: 'آروان', Icon: Globe },
]

export function Sidebar({
  provider,
  onChange,
  open,
  onClose,
}: {
  provider: Provider
  onChange: (p: Provider) => void
  open: boolean
  onClose: () => void
}) {
  const { mode, toggle } = useTheme()
  return (
    <>
      <div className={'cc-scrim' + (open ? ' on' : '')} onClick={onClose} aria-hidden="true" />
      <aside className={'cc-sidebar p-' + provider + (open ? ' open' : '')}>
        <div className="cc-brand">
          <div className="cc-brand-mark">CG</div>
          <div className="cc-brand-text">
            <div className="cc-brand-name">Cloud Guardian</div>
            <div className="cc-brand-sub">کنسول ابری</div>
          </div>
          <button type="button" className="cc-drawer-x" aria-label="بستن منو" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <nav className="cc-sidenav" aria-label="سرویس‌ها">
          <div className="cc-sidenav-label">سرویس‌ها</div>
          {PROVIDERS.map((it) => (
            <button
              key={it.id}
              type="button"
              className={'cc-sideitem p-' + it.id + (provider === it.id ? ' on' : '')}
              aria-current={provider === it.id ? 'page' : undefined}
              onClick={() => {
                onChange(it.id)
                onClose()
              }}
            >
              <it.Icon size={19} weight={provider === it.id ? 'fill' : 'regular'} />
              <span>{it.label}</span>
            </button>
          ))}
        </nav>

        <nav className="cc-sidenav" aria-label="تنظیمات">
          <div className="cc-sidenav-label">تنظیمات</div>
          <button type="button" className="cc-sideitem" onClick={toggle}>
            {mode === 'dark' ? <Sun size={19} /> : <Moon size={19} />}
            <span>{mode === 'dark' ? 'حالت روشن' : 'حالت تاریک'}</span>
          </button>
        </nav>

        <div className="cc-sidefoot">
          <div className="cc-profile">
            <div className="cc-profile-ava">م</div>
            <div className="cc-profile-meta">
              <div className="cc-profile-name">مدیر</div>
              <div className="cc-profile-role">دسترسی کامل</div>
            </div>
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
