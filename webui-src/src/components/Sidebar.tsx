import { Cloud, Cube, Globe, HardDrives, Moon, Sun } from '@phosphor-icons/react'
import { useTheme } from './theme'

export type Provider = 'hz' | 'cf' | 'ln' | 'arv'

const PROVIDERS: { id: Provider; label: string; Icon: typeof Cloud }[] = [
  { id: 'hz', label: 'هتزنر', Icon: HardDrives },
  { id: 'cf', label: 'کلادفلر', Icon: Cloud },
  { id: 'ln', label: 'لینود', Icon: Cube },
  { id: 'arv', label: 'آروان', Icon: Globe },
]

export function Sidebar({
  provider,
  onChange,
}: {
  provider: Provider
  onChange: (p: Provider) => void
}) {
  const { mode, toggle } = useTheme()
  return (
    <aside className="cc-sidebar">
      <div className="cc-brand">
        <div className="cc-brand-mark">CG</div>
        <div className="cc-brand-text">
          <div className="cc-brand-name">Cloud Guardian</div>
          <div className="cc-brand-sub">کنسول ابری</div>
        </div>
      </div>
      <nav className="cc-sidenav" aria-label="سرویس‌ها">
        <div className="cc-sidenav-label">سرویس‌ها</div>
        <button
          type="button"
          className={'cc-sideitem' + (provider === 'hz' ? ' on' : '')}
          aria-current={provider === 'hz' ? 'page' : undefined}
          onClick={() => onChange('hz')}
        >
          <HardDrives size={18} />
          <span>هتزنر</span>
        </button>
        <button
          type="button"
          className={'cc-sideitem' + (provider === 'cf' ? ' on' : '')}
          aria-current={provider === 'cf' ? 'page' : undefined}
          onClick={() => onChange('cf')}
        >
          <Cloud size={18} />
          <span>کلادفلر</span>
        </button>
        <button
          type="button"
          className={'cc-sideitem' + (provider === 'ln' ? ' on' : '')}
          aria-current={provider === 'ln' ? 'page' : undefined}
          onClick={() => onChange('ln')}
        >
          <Cube size={18} />
          <span>لینود</span>
        </button>
        <button
          type="button"
          className={'cc-sideitem' + (provider === 'arv' ? ' on' : '')}
          aria-current={provider === 'arv' ? 'page' : undefined}
          onClick={() => onChange('arv')}
        >
          <Globe size={18} />
          <span>آروان</span>
        </button>
      </nav>
      <div className="cc-sidefoot">
        <button type="button" className="cc-sideitem" onClick={toggle}>
          {mode === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
          <span>{mode === 'dark' ? 'حالت روشن' : 'حالت تاریک'}</span>
        </button>
      </div>
    </aside>
  )
}

export function ProviderRail({
  provider,
  onChange,
}: {
  provider: Provider
  onChange: (p: Provider) => void
}) {
  const { mode, toggle } = useTheme()
  return (
    <nav className="cc-provider-rail" aria-label="بخش‌های اصلی ربات">
      <div className="cc-rail-brand" aria-hidden="true">
        CG
      </div>
      {PROVIDERS.map((it) => (
        <button
          key={it.id}
          type="button"
          className={'cc-railitem' + (provider === it.id ? ' on' : '')}
          aria-label={it.label}
          title={it.label}
          aria-current={provider === it.id ? 'page' : undefined}
          onClick={() => onChange(it.id)}
        >
          <it.Icon size={20} weight={provider === it.id ? 'fill' : 'regular'} />
        </button>
      ))}
      <div className="cc-rail-sep" />
      <button
        type="button"
        className="cc-railitem cc-rail-theme"
        aria-label="تغییر تم"
        title={mode === 'dark' ? 'حالت روشن' : 'حالت تاریک'}
        onClick={toggle}
      >
        {mode === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
      </button>
    </nav>
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
