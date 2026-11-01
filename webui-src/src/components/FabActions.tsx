import { useState } from 'react'
import { CaretDown, Plus } from '@phosphor-icons/react'
import { Button } from './ui'
import type { PageAction } from './PageAction'

// One cluster for every viewport: labeled button(s) in the topbar on desktop,
// a circular "+" at the bottom of the screen on phones (sheet when there are
// several ways to add something).
export function ActionCluster({ actions }: { actions: PageAction[] }) {
  const [open, setOpen] = useState(false)
  if (actions.length === 0) return null
  const first = actions[0]
  const rest = actions.slice(1)
  const one = actions.length === 1

  return (
    <>
      <Button
        className="cc-pageaction"
        variant="primary"
        icon={first.icon}
        disabled={first.disabled}
        onClick={first.onClick}
      >
        {first.label}
      </Button>
      {!one && (
        <button type="button" className="cc-pageaction-more" aria-label="گزینه‌های افزودن" aria-haspopup="menu" onClick={() => setOpen(true)}>
          <CaretDown size={14} weight="bold" />
        </button>
      )}
      <button
        type="button"
        className="cc-fab"
        aria-label={one ? first.label : 'افزودن'}
        aria-haspopup={one ? undefined : 'menu'}
        disabled={one && first.disabled}
        onClick={() => (one ? first.onClick() : setOpen(true))}
      >
        <Plus size={26} weight="bold" />
      </button>
      {open && (
        <>
          <div className="cc-pop-scrim" onClick={() => setOpen(false)} aria-hidden="true" />
          <div className="cc-pop" role="menu" aria-label="افزودن">
            <div className="cc-pop-title">چه چیزی اضافه کنم؟</div>
            {actions.map((a) => (
              <button
                key={a.label}
                type="button"
                role="menuitem"
                className="cc-actionrow"
                disabled={a.disabled}
                onClick={() => {
                  setOpen(false)
                  a.onClick()
                }}
              >
                {a.icon && <a.icon size={18} />}
                <span>{a.label}</span>
              </button>
            ))}
          </div>
        </>
      )}
    </>
  )
}
