import * as React from 'react'
import * as DialogPrimitive from '@radix-ui/react-dialog'
import { cn } from './utils'

function DialogContent({
  className,
  children,
  size,
  ...props
}: React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & { size?: 'sm' | 'base' | 'lg' }) {
  // Dialogs are opened from state, not from Dialog.Trigger, so Radix has nothing to
  // restore focus to on close; remember the opener and put focus back ourselves.
  const prevFocus = React.useRef<HTMLElement | null>(
    typeof document !== 'undefined' ? (document.activeElement as HTMLElement | null) : null,
  )
  React.useEffect(() => {
    const el = prevFocus.current
    return () => {
      if (el && document.contains(el)) el.focus({ preventScroll: true })
    }
  }, [])
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/35" />
      <DialogPrimitive.Content
        className={cn(
          'fixed left-1/2 top-1/2 z-50 w-full max-w-[min(440px,92vw)] -translate-x-1/2 -translate-y-1/2 rounded-2xl border border-border bg-card text-foreground shadow-pop outline-none',
          size === 'lg' && 'max-w-[min(560px,94vw)]',
          className,
        )}
        onOpenAutoFocus={(e) => {
          // Never focus a field on open: on phones that pops the keyboard before the
          // user taps anything. Focus the dialog itself so Esc/scroll still work.
          e.preventDefault()
          const root = e.currentTarget as HTMLElement | null
          root?.focus({ preventScroll: true })
        }}
        {...props}
      >
        {children}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  )
}

export const Dialog = Object.assign(DialogContent, {
  Root: DialogPrimitive.Root,
  Trigger: DialogPrimitive.Trigger,
  Close: DialogPrimitive.Close,
  Title: DialogPrimitive.Title,
  Description: DialogPrimitive.Description,
})
