import { useEffect, useRef, useState, type MouseEvent } from 'react'
import { Check, Copy } from '@phosphor-icons/react'

export function CopyBtn({
  value,
  onCopy,
  label = 'کپی',
  className = '',
  size = 15,
}: {
  value: string
  onCopy: (v: string) => void | Promise<void>
  label?: string
  className?: string
  size?: number
}) {
  const [done, setDone] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => () => clearTimeout(timer.current), [])

  async function handle(e: MouseEvent<HTMLButtonElement>) {
    e.preventDefault()
    e.stopPropagation()
    if (!value) return
    await onCopy(value)
    setDone(true)
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setDone(false), 1400)
  }

  return (
    <button
      type="button"
      className={'cc-copybtn' + (done ? ' on' : '') + (className ? ' ' + className : '')}
      aria-label={done ? 'کپی شد' : label}
      title={label}
      onClick={handle}
    >
      {done ? <Check size={size} weight="bold" /> : <Copy size={size} />}
    </button>
  )
}
