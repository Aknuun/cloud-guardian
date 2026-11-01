import type { ReactNode } from 'react'

// One header pattern for every section: title, optional count, at most one action.
export function SectionHeader({
  title,
  count,
  action,
  className,
}: {
  title: string
  count?: ReactNode
  action?: ReactNode
  className?: string
}) {
  return (
    <div className={'cc-section-head' + (className ? ' ' + className : '')}>
      <div className="cc-section-title">
        <h2>{title}</h2>
        {count ? <span className="cc-sub">{count}</span> : null}
      </div>
      {action ? <span className="cc-section-action">{action}</span> : null}
    </div>
  )
}
