import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from './utils'

type IconType = React.ComponentType<{ size?: number | string; className?: string; weight?: any }>

const buttonVariants = cva(
  'inline-flex select-none items-center justify-center gap-1.5 whitespace-nowrap rounded-lg text-[13px] font-semibold transition-[filter,background-color,color,opacity] active:scale-[0.985] disabled:pointer-events-none disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40',
  {
    variants: {
      variant: {
        primary: 'bg-primary text-primary-foreground shadow-card hover:brightness-95',
        secondary: 'border border-border bg-card text-foreground hover:bg-secondary',
        outline: 'border border-border bg-transparent text-foreground hover:bg-secondary',
        ghost: 'bg-transparent text-muted-foreground hover:bg-secondary hover:text-foreground',
        destructive: 'bg-destructive text-white hover:brightness-95',
        danger: 'border border-destructive/30 bg-destructive/10 text-destructive hover:bg-destructive/15',
        secondaryGhost: 'bg-transparent text-muted-foreground hover:bg-secondary',
        'secondary-destructive': 'border border-destructive/25 bg-destructive/5 text-destructive hover:bg-destructive/10',
      },
      size: {
        sm: 'h-8 px-3 text-xs',
        base: 'h-11 px-4',
        lg: 'h-12 px-5 text-sm',
        default: 'h-11 px-4',
        icon: 'h-11 w-11 p-0',
      },
      shape: {
        default: '',
        square: 'aspect-square px-0',
      },
    },
    defaultVariants: { variant: 'secondary', size: 'default', shape: 'default' },
  },
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  icon?: IconType
  shape?: 'default' | 'square'
  size?: 'sm' | 'base' | 'lg' | 'default' | 'icon'
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, variant, size, shape, icon: Icon, children, ...props },
  ref,
) {
  const square = shape === 'square'
  return (
    <button
      ref={ref}
      className={cn(
        buttonVariants({ variant, size: square ? 'icon' : size, shape }),
        square && 'rounded-lg',
        className,
      )}
      {...props}
    >
      {Icon ? <Icon size={16} weight="bold" /> : null}
      {children}
    </button>
  )
})
