import { cn } from './utils'

export function Loader({ size = 'base' }: { size?: 'sm' | 'base' | 'lg' }) {
  const dim = size === 'lg' ? 'h-8 w-8 border-[3px]' : size === 'sm' ? 'h-4 w-4 border-2' : 'h-5 w-5 border-2'
  return (
    <div
      role="status"
      aria-label="در حال بارگذاری"
      className={cn('animate-spin rounded-full border-border border-t-primary', dim)}
    />
  )
}
