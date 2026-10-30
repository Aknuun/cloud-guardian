import * as React from 'react'
import { cn } from './utils'

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, id, name, ...props }, ref) {
    const auto = React.useId()
    return (
      <input
        ref={ref}
        id={id || auto}
        name={name || id || auto}
        className={cn(
          'flex h-11 w-full rounded-lg border border-border bg-card px-3 text-sm text-foreground placeholder:text-muted-foreground transition-colors focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/20 disabled:opacity-50',
          className,
        )}
        {...props}
      />
    )
  },
)
