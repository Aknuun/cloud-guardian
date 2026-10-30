import type { ReactNode } from 'react'
import { useState } from 'react'
import { ArrowClockwise, ArrowRight, CheckCircle, List, WarningCircle } from '@phosphor-icons/react'
import { AppVersion } from './AppVersion'
import { invalidateCache } from '../lib'
import { LogoutButton } from './LogoutButton'
import { useSectionTheme } from './sectionTheme'
import { Button } from './ui'
import { Sidebar, ThemeToggle } from './Sidebar'
import { toolItems } from './toolsMeta'
import { GlobalSearch } from './GlobalSearch'
import type { ToolId } from './toolsMeta'
import type { SubItem } from './Sidebar'
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
  sections,
  onTool,
  activeTool,
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
  sections?: SubItem[]
  onTool?: (id: ToolId) => void
  activeTool?: ToolId | null
  children: ReactNode
}) {
  const sectionTheme = useSectionTheme(provider)
  const [navOpen, setNavOpen] = useState(() => window.matchMedia('(min-width: 900px)').matches)
  return (
    <div className={"cc-app p-" + provider} style={sectionTheme.style}>
      <Sidebar
        provider={provider}
        onChange={onChange}
        open={navOpen}
        onToggle={() => setNavOpen((o) => !o)}
        sections={sections}
        tools={onTool ? toolItems(activeTool ?? null, onTool) : undefined}
        providerActive={!activeTool}
      />
      <div className="cc-content">
        <div className={'cc-progress' + (busy ? ' on' : '')} aria-hidden="true" />
        <header className="cc-topbar">
          <button type="button" className="cc-menu-btn" aria-label="منو" onClick={() => setNavOpen((o) => !o)}>
            <List size={20} />
          </button>
          {showBack && (
            <Button variant="ghost" shape="square" icon={ArrowRight} aria-label="بازگشت" onClick={onBack} />
          )}
          <h1 className="cc-title">{title}</h1>
          <GlobalSearch
            go={(p, t) => {
              if (t && onTool) onTool(t)
              else if (p) onChange(p)
            }}
          />
          <AppVersion />
          <ThemeToggle />
          <Button
            className={busy ? 'cc-spin' : ''}
            variant="ghost"
            shape="square"
            icon={ArrowClockwise}
            aria-label="بروزرسانی"
            onClick={() => {
              invalidateCache()
              onRefresh()
            }}
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
