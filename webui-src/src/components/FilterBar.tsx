import { useState } from 'react'
import { ArrowClockwise, Check, FunnelSimple, GearSix } from '@phosphor-icons/react'
import { Button, Input } from './ui'

export type FilterItem = { id: string; label: string }
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

// Page toolbar: search + filter menu + actions menu, mirroring the MoreBot pages.
export function FilterBar({ q, onQ, placeholder, filters, value, onValue, menu, onMenu, onRefresh, onEnter, busy }: Props) {
  const [open, setOpen] = useState<'f' | 'm' | null>(null)
  const current = filters?.find((f) => f.id === value)
  return (
    <div className="cc-filterbar">
      <div className="cc-searchbar">
        <Input
          value={q}
          onChange={(e) => onQ(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onEnter?.()}
          placeholder={placeholder || 'جستجو…'}
        />
        {filters && filters.length > 0 && (
          <Button
            variant="outline"
            icon={FunnelSimple}
            onClick={() => setOpen(open === 'f' ? null : 'f')}
          >
            {current && current.id !== filters[0].id ? current.label : 'فیلتر'}
          </Button>
        )}
        {menu && menu.length > 0 && (
          <Button variant="outline" icon={GearSix} onClick={() => setOpen(open === 'm' ? null : 'm')} aria-label="تنظیمات" />
        )}
        {onRefresh && (
          <Button variant="outline" icon={ArrowClockwise} onClick={onRefresh} disabled={busy} aria-label="تازه‌سازی" />
        )}
      </div>

      {open === 'f' && filters && (
        <>
          <div className="cc-menu-scrim" onClick={() => setOpen(null)} />
          <div className="cc-menu">
            {filters.map((f) => (
              <button
                key={f.id}
                type="button"
                className={'cc-menu-item' + (f.id === value ? ' on' : '')}
                onClick={() => {
                  onValue?.(f.id)
                  setOpen(null)
                }}
              >
                <span className="cc-grow">{f.label}</span>
                {f.id === value && <Check size={15} />}
              </button>
            ))}
          </div>
        </>
      )}

      {open === 'm' && menu && (
        <>
          <div className="cc-menu-scrim" onClick={() => setOpen(null)} />
          <div className="cc-menu">
            {menu.map((m) => (
              <button
                key={m.id}
                type="button"
                className={'cc-menu-item' + (m.danger ? ' danger' : '')}
                onClick={() => {
                  onMenu?.(m.id)
                  setOpen(null)
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
