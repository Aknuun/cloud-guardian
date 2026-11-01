import { useState } from 'react'
import { ArrowClockwise, Check, FunnelSimple } from '@phosphor-icons/react'
import { Button, Input } from './ui'
import { faNum } from '../lib'

export type FilterItem = { id: string; label: string; count?: number }
export type MenuItem = { id: string; label: string; danger?: boolean }

type Props = {
  q: string
  onQ: (v: string) => void
  placeholder?: string
  filters?: FilterItem[]
  value?: string
  onValue?: (id: string) => void
  menu?: MenuItem[]
  onMenu?: (id: string) => void
  onRefresh?: () => void
  onEnter?: () => void
  busy?: boolean
}

// Page toolbar: search + one menu holding both the filters and the page actions.
export function FilterBar({ q, onQ, placeholder, filters, value, onValue, menu, onMenu, onRefresh, onEnter, busy }: Props) {
  const [open, setOpen] = useState(false)
  const current = filters?.find((f) => f.id === value)
  const changed = current && filters && current.id !== filters[0].id
  const hasMenu = (filters && filters.length > 0) || (menu && menu.length > 0)
  return (
    <div className="cc-filterbar">
      <div className="cc-searchbar">
        <Input
          id="cc-q"
          name="q"
          aria-label={placeholder}
          value={q}
          onChange={(e) => onQ(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onEnter?.()}
          placeholder={placeholder || 'جستجو…'}
        />
        {hasMenu && (
          <Button
            variant="outline"
            icon={FunnelSimple}
            onClick={() => setOpen(!open)}
            aria-label="فیلتر و تنظیمات"
            aria-expanded={open}
          >
            {changed ? current!.label + (current!.count != null ? ' · ' + faNum(current!.count) : '') : 'فیلتر'}
          </Button>
        )}
        {onRefresh && (
          <Button variant="outline" icon={ArrowClockwise} onClick={onRefresh} disabled={busy} aria-label="تازه‌سازی" />
        )}
      </div>

      {open && hasMenu && (
        <>
          <div className="cc-menu-scrim" onClick={() => setOpen(false)} />
          <div className="cc-menu">
            {filters?.map((f) => (
              <button
                key={f.id}
                type="button"
                className={'cc-menu-item' + (f.id === value ? ' on' : '')}
                onClick={() => {
                  onValue?.(f.id)
                  setOpen(false)
                }}
              >
                <span className="cc-grow">{f.label}</span>
                {f.count != null && <span className="cc-fcount">{faNum(f.count)}</span>}
                {f.id === value && <Check size={15} />}
              </button>
            ))}
            {filters && filters.length > 0 && menu && menu.length > 0 && <div className="cc-menu-sep" />}
            {menu?.map((m) => (
              <button
                key={m.id}
                type="button"
                className={'cc-menu-item' + (m.danger ? ' danger' : '')}
                onClick={() => {
                  onMenu?.(m.id)
                  setOpen(false)
                }}
              >
                {m.label}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
