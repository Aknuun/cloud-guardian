import { useState } from 'react'
import type { ReactNode } from 'react'
import { DotsThreeVertical } from '@phosphor-icons/react'

/** Row for a list: exactly two lines (title + meta), optional trailing slot. */
export function ListRow({
  title,
  meta,
  lead,
  trailing,
  onClick,
  className,
}: {
  title: ReactNode
  meta?: ReactNode
  lead?: ReactNode
  trailing?: ReactNode
  onClick?: () => void
  className?: string
}) {
  const Tag = onClick ? 'button' : 'div'
  return (
    <Tag
      type={onClick ? 'button' : undefined}
      className={'cc-trow' + (onClick ? ' cc-tap' : '') + (className ? ' ' + className : '')}
      onClick={onClick}
    >
      {lead}
      <div className="cc-grow">
        <div className="cc-name">{title}</div>
        {meta ? <div className="cc-sub">{meta}</div> : null}
      </div>
      {trailing}
      {onClick ? <span className="cc-rowchev" aria-hidden="true">‹</span> : null}
    </Tag>
  )
}

/** Grouped settings block: one card, rows of label/value, optional footer. */
export function SettingsCard({
  title,
  action,
  children,
  footer,
}: {
  title?: string
  action?: ReactNode
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <div className="cc-setcard">
      {title || action ? (
        <div className="cc-setcard-head">
          {title ? <div className="cc-setcard-title">{title}</div> : null}
          {action}
        </div>
      ) : null}
      <div className="cc-setcard-body">{children}</div>
      {footer ? <div className="cc-setcard-foot">{footer}</div> : null}
    </div>
  )
}

export function SettingsRow({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children?: ReactNode
}) {
  return (
    <div className="cc-setrow">
      <div className="cc-setrow-label">
        <span>{label}</span>
        {hint ? <small>{hint}</small> : null}
      </div>
      {children ? <div className="cc-setrow-ctl">{children}</div> : null}
    </div>
  )
}

/** Filter chips: one value, or several when `value` is an array. */
export function ChipGroup({
  items,
  value,
  onChange,
}: {
  items: { id: string; label: string }[]
  value: string | string[]
  onChange: (id: string) => void
}) {
  return (
    <div className="cc-chips" role="group">
      {items.map((it) => {
        const on = Array.isArray(value) ? value.includes(it.id) : value === it.id
        return (
          <button
            key={it.id}
            type="button"
            className={'cc-chip' + (on ? ' on' : '')}
            aria-pressed={on}
            onClick={() => onChange(it.id)}
          >
            {it.label}
          </button>
        )
      })}
    </div>
  )
}

/** Row overflow: one ⋯ button hiding the secondary actions. */
export function RowMenu({ items, label = 'کارهای بیشتر' }: { items: { label: string; danger?: boolean; onClick: () => void }[]; label?: string }) {
  const [open, setOpen] = useState(false)
  if (!items.length) return null
  return (
    <div className="cc-menuwrap">
      <button type="button" className="cc-rowbtn" aria-label={label} aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <DotsThreeVertical size={18} />
      </button>
      {open && (
        <>
          <div className="cc-menu-scrim" onClick={() => setOpen(false)} />
          <div className="cc-menu" role="menu">
            {items.map((it) => (
              <button
                key={it.label}
                type="button"
                role="menuitem"
                className={'cc-menu-item' + (it.danger ? ' danger' : '')}
                onClick={() => {
                  setOpen(false)
                  it.onClick()
                }}
              >
                {it.label}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
