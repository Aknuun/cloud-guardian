import { useState } from 'react'
import type { ReactNode } from 'react'
import { CaretDown, CaretLeft, GearSix, House, Envelope, Lifebuoy, Broadcast, ArrowsClockwise, CalendarDots, BellSimple, Fire, GlobeSimple, Pulse, BellRinging, ShieldCheck, Tag, Terminal } from '@phosphor-icons/react'
import type { Icon } from '@phosphor-icons/react'
import { PROVIDERS } from './Sidebar'
import type { Provider } from './Sidebar'

export type ToolId = 'home' | 'srv' | 'rem' | 'pg' | 'um' | 'dom' | 'hf' | 'hfd' | 'hook' | 'mail' | 'set' | 'fo' | 'hub' | 'cfcheck' | 'hs'

// Hub is the author's own control room, not a feature of a normal install.
let hubOn = false
export function setHubOn(v: boolean) {
  hubOn = v
}
export function toolsVisible() {
  return hubOn ? TOOLS : TOOLS.filter((t) => t.id !== 'hub')
}

export const TOOLS: { id: ToolId; label: string; Icon: Icon }[] = [
  { id: 'pg', label: 'پاسارگارد و مانیتور نود', Icon: ShieldCheck },
  { id: 'hs', label: 'استودیوی نام هاست', Icon: Tag },
  { id: 'hook', label: 'هشدار انقضا و حجم', Icon: BellSimple },
  { id: 'hf', label: 'تعویض ساب‌فیلتر', Icon: ArrowsClockwise },
  { id: 'hfd', label: 'چرخش روزانه ساب‌ها', Icon: CalendarDots },
  { id: 'um', label: 'گزارش بد مصرف', Icon: Fire },
  { id: 'srv', label: 'سرورها و مانیتورینگ', Icon: Terminal },
  { id: 'dom', label: 'دامنه و SSL', Icon: GlobeSimple },
  { id: 'mail', label: 'ایمیل دامنه', Icon: Envelope },
  { id: 'cfcheck', label: 'سلامت ساب‌ها', Icon: Pulse },
  { id: 'fo', label: 'فیلور / پشتیبان', Icon: Lifebuoy },
  { id: 'hub', label: 'شبکهٔ نصب‌ها', Icon: Broadcast },
  { id: 'rem', label: 'یادآورها', Icon: BellRinging },
  { id: 'set', label: 'تنظیمات', Icon: GearSix },
]

// Sidebar + home share one order; the first item of each group is the important one.
export const TOOL_GROUPS: { title: string; tools: ToolId[] }[] = [
  { title: 'پاسارگارد', tools: ['pg', 'hs', 'hook', 'hf', 'hfd', 'um'] },
  { title: 'سرور و دامنه', tools: ['srv', 'dom', 'mail', 'cfcheck'] },
  { title: 'ابزارها', tools: ['hub', 'fo', 'rem', 'set'] },
]

// One collapsible block per group; open by default, remembered for the visit.
function HomeSection({ title, children }: { title: string; children: ReactNode }) {
  const [open, setOpen] = useState(true)
  return (
    <section className="cc-hg-sec">
      <button type="button" className="cc-hg-toggle" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <span>{title}</span>
        <CaretDown size={14} className={'cc-hg-caret' + (open ? ' on' : '')} />
      </button>
      {open && <div className="cc-hg-list">{children}</div>}
    </section>
  )
}

// Landing page: every ported section, grouped and one tap away.
export function HomeView({ go }: { go?: (p: Provider | null, t: ToolId | null) => void }) {
  const byId = new Map(toolsVisible().map((t) => [t.id, t]))
  return (
    <div className="cc-home2">
      <HomeSection title="سرویس‌ها">
        {PROVIDERS.map((p) => (
          <button key={p.id} className="cc-hrow" onClick={() => go!(p.id as Provider, null)}>
            <p.Icon size={17} />
            <span>{p.label}</span>
            <CaretLeft size={13} className="cc-hchev" />
          </button>
        ))}
      </HomeSection>
      {TOOL_GROUPS.map((g) => (
        <HomeSection key={g.title} title={g.title}>
          {g.tools.map((id) => {
            const t = byId.get(id)
            if (!t) return null
            return (
              <button key={id} className="cc-hrow" onClick={() => go!(null, id)}>
                <t.Icon size={17} />
                <span>{t.label}</span>
                <CaretLeft size={13} className="cc-hchev" />
              </button>
            )
          })}
        </HomeSection>
      ))}
    </div>
  )
}

export type ToolItem = { id: ToolId; label: string; Icon: Icon; active?: boolean; onSelect: () => void }

export function toolItems(active: ToolId | null, onSelect: (id: ToolId) => void): ToolItem[] {
  const all = [{ id: 'home' as ToolId, label: 'خانه', Icon: House }, ...toolsVisible()]
  return all.map((t) => ({ ...t, active: active === t.id, onSelect: () => onSelect(t.id) }))
}

export { FormDialog } from '../tools/shared'

export function ErrBox({ msg, onRetry }: { msg: string; onRetry?: () => void }) {
  return (
    <div className="cc-errbox" role="alert">
      <span>{msg}</span>
      {onRetry && (
        <button type="button" className="cc-errbox-retry" onClick={onRetry}>
          تلاش دوباره
        </button>
      )}
    </div>
  )
}

export function Skeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="cc-skel" aria-busy="true" aria-live="polite" aria-label="در حال بارگذاری">
      <div className="cc-skel-stats">
        {[0, 1, 2].map((i) => <span key={i} className="cc-skel-tile" />)}
      </div>
      {Array.from({ length: rows }).map((_, i) => <span key={i} className="cc-skel-row" />)}
    </div>
  )
}
