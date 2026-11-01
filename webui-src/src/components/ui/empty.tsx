import type * as React from 'react'
export function Empty({ title, description, action }: { title: string; description?: string; action?: React.ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center gap-1 px-4 py-12 text-center">
      <div className="text-sm font-semibold text-foreground">{title}</div>
      {description ? <div className="max-w-[32ch] text-xs leading-6 text-muted-foreground">{description}</div> : null}
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  )
}
