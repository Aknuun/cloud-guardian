import type { ReactNode } from 'react'
import { CaretLeft, Trash } from '@phosphor-icons/react'

// One account tile, shared by Cloudflare and Hetzner so both lists read the same.
export function AccountCard({
  icon,
  name,
  meta,
  go,
  onOpen,
  onDelete,
  delLabel,
}: {
  icon: ReactNode
  name: string
  meta: string
  go: string
  onOpen: () => void
  onDelete: () => void
  delLabel: string
}) {
  return (
    <div
      className="cc-acctcard"
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onOpen()
        }
      }}
    >
      <span className="cc-acctcard-ico">{icon}</span>
      <span className="cc-grow">
        <span className="cc-name">{name}</span>
        <span className="cc-sub">{meta}</span>
      </span>
      <button
        type="button"
        className="cc-rowbtn danger"
        aria-label={delLabel}
        onClick={(e) => {
          e.stopPropagation()
          onDelete()
        }}
      >
        <Trash size={16} />
      </button>
      <span className="cc-acctcard-go">
        {go}
        <CaretLeft size={13} weight="bold" />
      </span>
    </div>
  )
}
