export function Empty({ title, description }: { title: string; description?: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-1 px-4 py-12 text-center">
      <div className="text-sm font-semibold text-foreground">{title}</div>
      {description ? <div className="max-w-[32ch] text-xs leading-6 text-muted-foreground">{description}</div> : null}
    </div>
  )
}
