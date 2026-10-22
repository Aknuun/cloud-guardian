import { Cloud, HardDrives, Moon, Sun } from '@phosphor-icons/react'
import { useTheme } from './theme'

export function Sidebar({
  provider,
  onChange,
}: {
  provider: 'hz' | 'cf'
  onChange: (p: 'hz' | 'cf') => void
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

export function ThemeToggle() {
  const { mode, toggle } = useTheme()
  return (
    <button type="button" className="cc-themebtn" aria-label="تغییر تم" onClick={toggle}>
      {mode === 'dark' ? <Sun size={19} /> : <Moon size={19} />}
    </button>
  )
}
