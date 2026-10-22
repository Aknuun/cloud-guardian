import type { ReactNode } from 'react'
import { ArrowClockwise, ArrowRight, CheckCircle, WarningCircle } from '@phosphor-icons/react'
import { AppVersion } from './AppVersion'
import { LogoutButton } from './LogoutButton'
import { Button } from './ui'
import { ProviderRail, Sidebar, ThemeToggle } from './Sidebar'
import type { Provider } from './Sidebar'

export function ProviderShell({
  provider,
  title,
  busy,
  showBack,
  showSwitch = true,
  onBack,
  onRefresh,
  onChange,
  toast,
  children,
}: {
  provider: Provider
  title: string
  busy: boolean
  showBack: boolean
  showSwitch?: boolean
  onBack: () => void
  onRefresh: () => void
  onChange: (p: Provider) => void
  toast: { t: string; err?: boolean } | null
  children: ReactNode
}) {
  return (
    <div className={"cc-app p-" + provider}>
      <Sidebar provider={provider} onChange={onChange} />
      <ProviderRail provider={provider} onChange={onChange} />
      <div className="cc-content">
        <div className={'cc-progress' + (busy ? ' on' : '')} aria-hidden="true" />
        <header className="cc-topbar">
          {showBack && (
            <Button variant="ghost" shape="square" icon={ArrowRight} aria-label="بازگشت" onClick={onBack} />
          )}
          <h1 className="cc-title">{title}</h1>
          <AppVersion />
          <ThemeToggle />
          <Button
            className={busy ? 'cc-spin' : ''}
            variant="ghost"
            shape="square"
            icon={ArrowClockwise}
            aria-label="بروزرسانی"
            onClick={onRefresh}
          />
          <LogoutButton />
        </header>
        <main className="cc-main" aria-busy={busy}>
          {showSwitch && (
            <div className="cc-mobile-only">
              <ProviderSwitch provider={provider} onChange={onChange} />
            </div>
          )}
          {children}
        </main>
        {toast && (
          <div className={'cc-toast ' + (toast.err ? 'err' : 'ok')} role="status" aria-live="polite">
            {toast.err ? <WarningCircle size={18} /> : <CheckCircle size={18} />}
            <span>{toast.t}</span>
          </div>
        )}
      </div>
    </div>
  )
}

export function ProviderSwitch({
  provider,
  onChange,
}: {
  provider: Provider
  onChange: (p: Provider) => void
}) {
  const items: { id: Provider; label: string }[] = [
    { id: 'hz', label: 'هتزنر' },
    { id: 'cf', label: 'کلادفلر' },
    { id: 'ln', label: 'لینود' },
    { id: 'arv', label: 'آروان' },
  ]
  return (
    <div className="cc-provider" role="group" aria-label="انتخاب سرویس">
      {items.map((it) => (
        <button
          key={it.id}
          className={'cc-provider-btn p-' + it.id + (provider === it.id ? ' on' : '')}
          aria-pressed={provider === it.id}
          onClick={() => onChange(it.id)}
        >
          {it.label}
        </button>
      ))}
    </div>
  )
}
