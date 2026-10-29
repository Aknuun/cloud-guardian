import * as React from 'react'
import * as DialogPrimitive from '@radix-ui/react-dialog'
import { cn } from './utils'

function DialogContent({
  className,
  children,
  size,
  ...props
}: React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & { size?: 'sm' | 'base' | 'lg' }) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/35 backdrop-blur-[2px] data-[state=open]:animate-fade-in data-[state=closed]:animate-fade-out" />
      <DialogPrimitive.Content
        className={cn(
          'fixed left-1/2 top-1/2 z-50 w-full max-w-[min(440px,92vw)] -translate-x-1/2 -translate-y-1/2 rounded-2xl border border-border bg-card text-foreground shadow-pop outline-none data-[state=open]:animate-dialog-in data-[state=closed]:animate-dialog-out',
          size === 'lg' && 'max-w-[min(560px,94vw)]',
          className,
        )}
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
