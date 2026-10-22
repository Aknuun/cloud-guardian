import * as React from 'react'
import { cn } from './utils'

export function LayerCard({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('rounded-xl border border-border bg-card text-card-foreground shadow-card', className)} {...props} />
}

export function Card(props: React.HTMLAttributes<HTMLDivElement>) {
  return <LayerCard {...props} />
}
