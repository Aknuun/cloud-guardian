import { useEffect, useState } from 'react'
import { BellRinging, ShieldCheck, Terminal } from '@phosphor-icons/react'
import type { Icon } from '@phosphor-icons/react'
import { api, faDate } from '../lib'
import { Badge, Loader, Empty } from './ui'
import { StatGrid } from './Stats'
import { faNum } from '../lib'

export type ToolId = 'srv' | 'rem' | 'pg'

export const TOOLS: { id: ToolId; label: string; Icon: Icon }[] = [
  { id: 'srv', label: 'سرورها و مانیتورینگ', Icon: Terminal },
  { id: 'rem', label: 'یادآورها', Icon: BellRinging },
  { id: 'pg', label: 'پاسارگارد', Icon: ShieldCheck },
]

export type ToolItem = { id: ToolId; label: string; Icon: Icon; active?: boolean; onSelect: () => void }

export function toolItems(active: ToolId | null, onSelect: (id: ToolId) => void): ToolItem[] {
  return TOOLS.map((t) => ({ ...t, active: active === t.id, onSelect: () => onSelect(t.id) }))
}

function useTool<T>(path: string) {
  const [data, setData] = useState<T | null>(null)
  const [err, setErr] = useState('')
  const [loaded, setLoaded] = useState(false)
  useEffect(() => {
    let stop = false
    api<T>(path)
      .then((d) => {
        if (stop) return
        setData(d)
        setLoaded(true)
      })
      .catch((e) => {
        if (stop) return
        setErr(e.message || 'خطا')
        setLoaded(true)
      })
    return () => {
      stop = true
    }
  }, [path])
  return { data, err, loaded }
}

type Srv = { id: string; name: string; host: string; user: string; port: number; note: string; key: boolean }
type Rem = { id: string; text: string; when: number; notified: boolean; ack: boolean; server: string }
type PgData = {
  panels: { name: string; url: string; user: string }[]
  monitors: { id: string; name: string; panel: string; interval: number; enabled: boolean }[]
}

export function Tools({ id }: { id: ToolId }) {
  if (id === 'srv') return <SrvView />
  if (id === 'rem') return <RemView />
  return <PgView />
}

function SrvView() {
  const { data, err, loaded } = useTool<Srv[]>('/api/tools/srv')
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const list = data || []
  return (
    <>
      <StatGrid
        items={[
          { label: 'سرورها', value: faNum(list.length) },
          { label: 'با کلید', value: faNum(list.filter((s) => s.key).length) },
          { label: 'با رمز', value: faNum(list.filter((s) => !s.key).length) },
        ]}
      />
      {list.length === 0 && <Empty title="سروری ثبت نشده" description="از ربات یک سرور SSH اضافه کن." />}
      {list.map((s) => (
        <div key={s.id} className="cc-trow">
          <div className="cc-avatar">{(s.name || 'S').slice(0, 1).toUpperCase()}</div>
          <div className="cc-grow">
            <div className="cc-name">{s.name || s.host}</div>
            <div className="cc-sub">
              {s.user}@{s.host}:{s.port}
            </div>
            {s.note && <div className="cc-sub">{s.note}</div>}
          </div>
          <Badge>{s.key ? 'کلید' : 'رمز'}</Badge>
        </div>
      ))}
    </>
  )
}

function RemView() {
  const { data, err, loaded } = useTool<Rem[]>('/api/tools/rem')
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const list = data || []
  const pending = list.filter((r) => !r.ack).length
  return (
    <>
      <StatGrid
        items={[
          { label: 'یادآورها', value: faNum(list.length) },
          { label: 'در انتظار', value: faNum(pending) },
          { label: 'انجام‌شده', value: faNum(list.length - pending) },
        ]}
      />
      {list.length === 0 && <Empty title="یادآوری نیست" description="از ربات یادآور بساز." />}
      {list.map((r) => (
        <div key={r.id} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{r.text}</div>
            <div className="cc-sub">
              {faDate(Number(r.when) * 1000)}
              {r.server ? ' · ' + r.server : ''}
            </div>
          </div>
          <Badge>{r.ack ? 'انجام شد' : r.notified ? 'اطلاع داده شد' : 'در انتظار'}</Badge>
        </div>
      ))}
    </>
  )
}

function PgView() {
  const { data, err, loaded } = useTool<PgData>('/api/tools/pg')
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const iso = data || { panels: [], monitors: [] }
  const on = iso.monitors.filter((m) => m.enabled).length
  return (
    <>
      <StatGrid
        items={[
          { label: 'پنل‌ها', value: faNum(iso.panels.length) },
          { label: 'مانیتور نود', value: faNum(iso.monitors.length) },
          { label: 'فعال', value: faNum(on) },
        ]}
      />
      <div className="cc-section">پنل‌ها</div>
      {iso.panels.length === 0 && <Empty title="پنلی ثبت نشده" description="از ربات پنل پاسارگارد اضافه کن." />}
      {iso.panels.map((p) => (
        <div key={p.url} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{p.name || p.url}</div>
            <div className="cc-sub cc-mono">{p.url}</div>
          </div>
          {p.user && <Badge>{p.user}</Badge>}
        </div>
      ))}
      <div className="cc-section">مانیتور نودها</div>
      {iso.monitors.length === 0 && <Empty title="مانیتوری نیست" description="از ربات مانیتور نود بساز." />}
      {iso.monitors.map((m) => (
        <div key={m.id} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{m.name || m.id}</div>
            <div className="cc-sub">
              {m.panel} · هر {faNum(m.interval)} دقیقه
            </div>
          </div>
          <Badge>{m.enabled ? 'فعال' : 'خاموش'}</Badge>
        </div>
      ))}
    </>
  )
}
