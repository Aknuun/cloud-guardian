import { CaretLeft, GearSix, House, Envelope, Lifebuoy, Broadcast, ArrowsClockwise, CalendarDots, BellSimple, Fire, GlobeSimple, Pulse, BellRinging, ShieldCheck, Tag, Terminal } from '@phosphor-icons/react'
import type { Icon } from '@phosphor-icons/react'
import { PROVIDERS } from './Sidebar'
import type { Provider } from './Sidebar'

export type ToolId = 'home' | 'srv' | 'rem' | 'pg' | 'um' | 'dom' | 'hf' | 'hfd' | 'hook' | 'mail' | 'set' | 'fo' | 'hub' | 'cfcheck' | 'hs'

export const TOOLS: { id: ToolId; label: string; Icon: Icon }[] = [
  { id: 'hs', label: 'استودیوی نام هاست', Icon: Tag },
  { id: 'pg', label: 'پاسارگارد', Icon: ShieldCheck },
  { id: 'hook', label: 'هشدار انقضا و حجم', Icon: BellSimple },
  { id: 'hf', label: 'تعویض خودکار ساب‌فیلتر', Icon: ArrowsClockwise },
  { id: 'hfd', label: 'چرخش روزانه ساب‌ها', Icon: CalendarDots },
  { id: 'um', label: 'گزارش بد مصرف', Icon: Fire },
  { id: 'srv', label: 'سرورها و مانیتورینگ', Icon: Terminal },
  { id: 'dom', label: 'دامنه و SSL', Icon: GlobeSimple },
  { id: 'rem', label: 'یادآورها', Icon: BellRinging },
  { id: 'mail', label: 'ایمیل دامنه', Icon: Envelope },
  { id: 'cfcheck', label: 'بررسی سلامت رکوردها', Icon: Pulse },
  { id: 'fo', label: 'فیلور / پشتیبان', Icon: Lifebuoy },
  { id: 'hub', label: 'مرکز هاب', Icon: Broadcast },
  { id: 'set', label: 'تنظیمات', Icon: GearSix },
]

// Sidebar + home share one order; the first item of each group is the important one.
export const TOOL_GROUPS: { title: string; tools: ToolId[] }[] = [
  { title: 'پاسارگارد', tools: ['hs', 'pg', 'hook', 'hf', 'hfd', 'um'] },
  { title: 'سرور و دامنه', tools: ['srv', 'dom', 'rem', 'mail', 'cfcheck'] },
  { title: 'ابزارها', tools: ['hub', 'fo', 'set'] },
]

// Landing page: every ported section, grouped and one tap away.
export function HomeView({ go }: { go?: (p: Provider | null, t: ToolId | null) => void }) {
  const byId = new Map(TOOLS.map((t) => [t.id, t]))
  const groups = TOOL_GROUPS
  return (
    <div className="cc-hg">
      <div className="cc-hg-title">سرویس‌ها</div>
      <div className="cc-hg-list">
        {PROVIDERS.map((p) => (
          <button key={p.id} className="cc-hrow" onClick={() => go!(p.id as Provider, null)}>
            <p.Icon size={18} />
            <span>{p.label}</span>
            <CaretLeft size={14} className="cc-hchev" />
          </button>
        ))}
      </div>
      {groups.map((g) => (
        <div key={g.title}>
          <div className="cc-hg-title">{g.title}</div>
          <div className="cc-hg-list">
            {g.tools.map((id) => {
              const t = byId.get(id)
              if (!t) return null
              return (
                <button key={id} className="cc-hrow" onClick={() => go!(null, id)}>
                  <t.Icon size={18} />
                  <span>{t.label}</span>
                  <CaretLeft size={14} className="cc-hchev" />
                </button>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}

export type ToolItem = { id: ToolId; label: string; Icon: Icon; active?: boolean; onSelect: () => void }

export function toolItems(active: ToolId | null, onSelect: (id: ToolId) => void): ToolItem[] {
  const all = [{ id: 'home' as ToolId, label: 'خانه', Icon: House }, ...TOOLS]
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
