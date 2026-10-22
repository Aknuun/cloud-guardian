import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from './utils'

const badgeVariants = cva(
  'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-medium leading-5',
  {
    variants: {
      variant: {
        secondary: 'border-border bg-secondary text-muted-foreground',
        neutral: 'border-border bg-secondary text-muted-foreground',
        success: 'border-transparent bg-[color-mix(in_srgb,var(--ds-success)_14%,transparent)] text-[var(--ds-success)]',
        error: 'border-transparent bg-[color-mix(in_srgb,var(--ds-danger)_13%,transparent)] text-[var(--ds-danger)]',
        warning: 'border-transparent bg-[color-mix(in_srgb,var(--ds-warning)_15%,transparent)] text-[var(--ds-warning)]',
        primary: 'border-transparent bg-[var(--ds-brand-soft)] text-[var(--ds-brand)]',
      },
    },
    defaultVariants: { variant: 'secondary' },
  },
)

export function Badge({
  className,
  variant,
  dot,
  children,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants> & { dot?: boolean }) {
  return (
    <span className={cn(badgeVariants({ variant }), className)} {...props}>
      {dot ? <span className="cc-badge-dot" aria-hidden="true" /> : null}
      {children}
    </span>
  )
}
