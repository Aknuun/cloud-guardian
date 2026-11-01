import type { ReactNode } from 'react'

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
  children,
  footer,
}: {
  title?: string
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <div className="cc-setcard">
      {title ? <div className="cc-setcard-title">{title}</div> : null}
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
  children: ReactNode
}) {
  return (
    <div className="cc-setrow">
      <div className="cc-setrow-label">
        <span>{label}</span>
        {hint ? <small>{hint}</small> : null}
      </div>
      <div className="cc-setrow-ctl">{children}</div>
    </div>
  )
}

/** Mutually exclusive filter chips. */
export function ChipGroup({
  items,
  value,
  onChange,
}: {
  items: { id: string; label: string }[]
  value: string
  onChange: (id: string) => void
}) {
  return (
    <div className="cc-chips" role="group">
      {items.map((it) => (
        <button
          key={it.id}
          type="button"
          className={'cc-chip' + (value === it.id ? ' on' : '')}
          aria-pressed={value === it.id}
          onClick={() => onChange(it.id)}
        >
          {it.label}
        </button>
      ))}
    </div>
  )
}
