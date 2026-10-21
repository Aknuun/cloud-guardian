import { useEffect, useRef, useState } from 'react'
import { Badge, Button, Dialog, Empty, Input, LayerCard, Loader } from '@cloudflare/kumo'
import { ArrowClockwise, ArrowRight, Star } from '@phosphor-icons/react'
import { api, fallbackCopy, faNum, num, TG } from './lib'

/* ---------------- types ---------------- */

type CFAcc = { i: number; name: string; zones?: number }
type CFZone = { id: string; name: string; status: string; acc: number; accName: string }
type CFRec = {
  id: string
  type: string
  name: string
  content: string
  ttl: number
  proxied: boolean
  priority?: number | null
  label: string
  fav: boolean
}
type ZoneInfo = {
  id: string
  name: string
  status: string
  paused: boolean
  plan: string
  created: string
  account: string
  ns: string[]
}
type SetItem = { key: string; label: string; value: string; kind: 'bool' | 'enum'; values?: { v: string; l: string }[] }
type SetGroup = { key: string; title: string; items: SetItem[] }
type EmailRule = { id: string; label: string; dest: string; enabled?: boolean }
type EmailData = {
  available: boolean
  enabled: boolean
  status?: string
  rules: EmailRule[]
  catchAll: string
  dests: { email: string; verified: boolean }[]
}
type FavItem = {
  acc: number
  zid: string
  rid: string
  zone: string
  label: string
  type?: string
  content?: string
  name?: string
  missing: boolean
}
type LbGroup = { name: string; on: number; off: number; total: number }
type LbEntry = {
  on: boolean
  id: string
  type: string
  content: string
  proxied: boolean
  ttl: number
  weight: number
  spare?: { type: string; content: string }
}
type View =
  | { v: 'zones' }
  | { v: 'accounts' }
  | { v: 'favs' }
  | { v: 'search' }
  | { v: 'zone'; acc: number; zid: string }
  | { v: 'lbzones' }
  | { v: 'lb'; acc: number; zid: string }
  | { v: 'lbgroup'; acc: number; zid: string; name: string }
type Field = {
  key: string
  label: string
  placeholder?: string
  value?: string
  password?: boolean
  select?: { value: string; label: string }[]
}
type PickItem = { id: string; title: string; sub?: string }
type Dlg =
  | { k: 'confirm'; title: string; desc?: string; danger?: boolean; okText?: string; onOk: () => void | Promise<void> }
  | { k: 'form'; title: string; desc?: string; fields: Field[]; okText?: string; onSubmit: (v: Record<string, string>) => void | Promise<void> }
  | { k: 'pick'; title: string; items: PickItem[]; onPick: (id: string) => void }
  | { k: 'list'; title: string; rows: { title: string; sub?: string }[]; empty?: string }
  | { k: 'record'; acc: number; zid: string; zone: string; rec: CFRec }
  | { k: 'editor'; acc: number; zid: string; zone: string; rec?: CFRec }
  | { k: 'spare'; acc: number; zid: string; name: string; entry: LbEntry }

const ZONE_BADGE: Record<string, any> = {
  active: 'success',
  paused: 'warning',
  pending: 'warning',
  initializing: 'secondary',
  moved: 'neutral',
  deleted: 'error',
}
const TTL_STEPS = [1, 120, 300, 1800, 3600, 21600, 86400]
const TYPES = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'SRV']
const PROXYABLE = ['A', 'AAAA', 'CNAME']

function statusLabel(s: string) {
  switch (s) {
    case 'active':
      return 'فعال'
    case 'paused':
      return 'متوقف'
    case 'pending':
      return 'در انتظار'
    case 'initializing':
      return 'راه‌اندازی'
    case 'moved':
      return 'منتقل‌شده'
    case 'deleted':
      return 'حذف‌شده'
  }
  return s
}
function ttlLabel(t: number) {
  if (!t || t === 1) return 'خودکار'
  if (t < 3600) return faNum(t / 60) + ' دقیقه'
  return faNum(t / 3600) + ' ساعت'
}
function relName(name: string, zone: string) {
  if (!zone) return name
  if (name === zone) return '@'
  if (name.endsWith('.' + zone)) return name.slice(0, -(zone.length + 1))
  return name
}

export default function Cloudflare({ onSwitch, onBackRef }: { onSwitch: () => void; onBackRef: (f: () => void) => void }) {
  const [view, setView] = useState<View>({ v: 'zones' })
  const [stack, setStack] = useState<View[]>([])
  const [rev, setRev] = useState(0)
  const [busy, setBusy] = useState(0)
  const [toastMsg, setToastMsg] = useState<{ t: string; err?: boolean } | null>(null)
  const [dlg, setDlg] = useState<Dlg | null>(null)
  const [q, setQ] = useState('')
  const [tab, setTab] = useState<'records' | 'settings' | 'email' | 'traffic'>('records')
  const [err, setErr] = useState('')

  const [zones, setZones] = useState<CFZone[]>([])
  const [accs, setAccs] = useState<CFAcc[]>([])
  const [accFilter, setAccFilter] = useState<number>(-1)

  const [zone, setZone] = useState<ZoneInfo | null>(null)
  const [records, setRecords] = useState<CFRec[]>([])
  const [sel, setSel] = useState<Set<string>>(new Set())
  const [selMode, setSelMode] = useState(false)

  const [settings, setSettings] = useState<{ groups: SetGroup[]; dev: boolean; paused: boolean } | null>(null)
  const [email, setEmail] = useState<EmailData | null>(null)
  const [traffic, setTraffic] = useState<{ days: number; total: number; top: { name: string; count: number }[] } | null>(null)
  const [favs, setFavs] = useState<FavItem[]>([])
  const [search, setSearch] = useState<{ zones: CFZone[]; records: (CFRec & { acc: number; zid: string; zone: string })[] } | null>(null)
  const [lbGroups, setLbGroups] = useState<LbGroup[]>([])
  const [lbName, setLbName] = useState('')
  const [lbEntries, setLbEntries] = useState<LbEntry[]>([])

  const [pendingRid, setPendingRid] = useState('')
  const [pendingEditor, setPendingEditor] = useState<{ acc: number; zid: string; zone: string } | null>(null)
  const [days, setDays] = useState(1)

  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const bump = (n: number) => setBusy((b) => Math.max(0, b + n))
  function toast(t: string, isErr = false) {
    setToastMsg({ t, err: isErr })
    clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToastMsg(null), 3400)
  }
  async function copy(v: string, label = 'کپی شد') {
    try {
      if (navigator.clipboard) await navigator.clipboard.writeText(v)
      else fallbackCopy(v)
      toast(label)
    } catch {
      fallbackCopy(v)
      toast(label)
    }
  }

  /* ---------------- boot ---------------- */

  useEffect(() => {
    if (TG) {
      TG.ready()
      TG.expand()
      TG.disableVerticalSwipes && TG.disableVerticalSwipes()
      TG.MainButton && TG.MainButton.hide()
    }
    return () => clearTimeout(toastTimer.current)
  }, [])

  /* ---------------- navigation ---------------- */

  function push(v: View) {
    setStack((s) => [...s, view])
    setView(v)
    setQ('')
    setErr('')
    if (v.v === 'zone') {
      setTab('records')
      setSel(new Set())
      setSelMode(false)
      setSettings(null)
      setEmail(null)
      setTraffic(null)
    }
  }
  function goBack() {
    if (dlg) {
      setDlg(null)
      return
    }
    if (stack.length) {
      setView(stack[stack.length - 1])
      setStack(stack.slice(0, -1))
      setQ('')
      setErr('')
      return
    }
    onSwitch()
  }
  onBackRef(goBack)
  useEffect(() => {
    if (TG && TG.BackButton) TG.BackButton.show()
  }, [view, stack.length, dlg])

  /* ---------------- data ---------------- */

  useEffect(() => {
    let cancelled = false
    setErr('')
    bump(1)
    ;(async () => {
      try {
        if (view.v === 'zones' || view.v === 'lbzones') {
          const d = await api<{ zones: CFZone[]; accounts: CFAcc[] }>('/api/cf/zones')
          if (!cancelled) {
            setZones(d.zones || [])
            setAccs(d.accounts || [])
          }
        } else if (view.v === 'accounts') {
          const d = await api<{ accounts: CFAcc[] }>('/api/cf/accounts')
          if (!cancelled) setAccs(d.accounts || [])
        } else if (view.v === 'favs') {
          const d = await api<{ items: FavItem[] }>('/api/cf/favs')
          if (!cancelled) setFavs(d.items || [])
        } else if (view.v === 'zone') {
          const d = await api<{ zone: ZoneInfo; records: CFRec[] }>(
            '/api/cf/records?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid),
          )
          if (cancelled) return
          setZone(d.zone)
          setRecords(d.records || [])
          if (pendingRid) {
            const hit = (d.records || []).find((r) => r.id === pendingRid)
            if (hit) setDlg({ k: 'record', acc: view.acc, zid: view.zid, zone: d.zone.name, rec: hit })
            setPendingRid('')
          }
          if (pendingEditor) {
            setDlg({ k: 'editor', acc: pendingEditor.acc, zid: pendingEditor.zid, zone: pendingEditor.zone })
            setPendingEditor(null)
          }
          if (tab === 'settings') {
            const s = await api<any>('/api/cf/settings?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid))
            if (!cancelled) setSettings({ groups: s.groups || [], dev: !!s.dev, paused: !!s.paused })
          } else if (tab === 'email') {
            const e = await api<EmailData>('/api/cf/email?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid))
            if (!cancelled) setEmail(e)
          } else if (tab === 'traffic') {
            const t = await api<any>(
              '/api/cf/traffic?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid) + '&days=' + days,
            )
            if (!cancelled) setTraffic({ days: t.days, total: t.total, top: t.top || [] })
          }
        } else if (view.v === 'lb') {
          const d = await api<{ zone: ZoneInfo; groups: LbGroup[] }>(
            '/api/cf/lbgroups?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid),
          )
          if (!cancelled) {
            setZone(d.zone)
            setLbGroups(d.groups || [])
          }
        } else if (view.v === 'lbgroup') {
          const d = await api<{ zone: ZoneInfo; name: string; entries: LbEntry[] }>(
            '/api/cf/lb?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid) + '&name=' + encodeURIComponent(view.name),
          )
          if (!cancelled) {
            setZone(d.zone)
            setLbName(d.name)
            setLbEntries(d.entries || [])
          }
        }
      } catch (e: any) {
        if (!cancelled) {
          setErr(e.message || 'خطا')
          toast(e.message || 'خطا', true)
        }
      } finally {
        bump(-1)
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, rev, tab, days, pendingRid])

  function refresh() {
    setRev((r) => r + 1)
  }
  useEffect(() => {
    if (view.v !== 'search') return
    const term = q.trim()
    if (term.length < 2) {
      setSearch(null)
      return
    }
    let cancelled = false
    const t = setTimeout(async () => {
      bump(1)
      try {
        const d = await api<{ zones: CFZone[]; records: any[] }>('/api/cf/search?q=' + encodeURIComponent(term))
        if (!cancelled) setSearch({ zones: d.zones || [], records: d.records || [] })
      } catch (e: any) {
        if (!cancelled) toast(e.message || 'خطا', true)
      } finally {
        bump(-1)
      }
    }, 350)
    return () => {
      cancelled = true
      clearTimeout(t)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, q])


  async function post(path: string, body: any, keep = false) {
    bump(1)
    try {
      const d = await api<any>(path, body)
      toast(d.message || 'انجام شد')
      setRev((r) => r + 1)
      if (!keep) setDlg(null)
      return d
    } catch (e: any) {
      toast(e.message || 'خطا', true)
      return null
    } finally {
      bump(-1)
    }
  }

  /* ---------------- record actions ---------------- */

  function recBody(rec: CFRec, acc: number, zid: string) {
    return { acc, zid, rid: rec.id }
  }

  async function toggleFav(acc: number, zid: string, rec: CFRec) {
    const d = await post('/api/cf/record', { ...recBody(rec, acc, zid), op: 'fav' })
    if (d) setDlg(null)
  }

  async function toggleProxy(acc: number, zid: string, rec: CFRec) {
    const d = await post('/api/cf/record', { ...recBody(rec, acc, zid), op: 'proxy', proxied: !rec.proxied }, true)
    if (d && d.record) setDlg((x) => (x && x.k === 'record' ? { ...x, rec: { ...x.rec, ...d.record } } : x))
  }

  async function cycleTTL(acc: number, zid: string, rec: CFRec) {
    const d = await post('/api/cf/record', { ...recBody(rec, acc, zid), op: 'ttl', ttl: 0 }, true)
    if (d && d.record) setDlg((x) => (x && x.k === 'record' ? { ...x, rec: { ...x.rec, ...d.record } } : x))
  }

  async function showRecordTraffic(acc: number, zid: string, rec: CFRec, zname: string) {
    bump(1)
    try {
      const t = await api<any>('/api/cf/traffic?acc=' + acc + '&zid=' + encodeURIComponent(zid) + '&days=1')
      const counts: Record<string, number> = t.counts || {}
      const fq = (rec.name || '').toLowerCase().replace(/\.$/, '')
      const rel = fq === zname ? zname : fq
      const n = counts[rel] || counts[fq] || 0
      setDlg({
        k: 'list',
        title: '📊 ترافیک امروز',
        rows: [
          { title: rec.label, sub: 'کوئری DNS امروز' },
          { title: faNum(n) + ' کوئری', sub: 'مجموع دامنه: ' + faNum(t.total || 0) },
        ],
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  async function showRefs(acc: number, zid: string, rec: CFRec) {
    bump(1)
    try {
      const d = await api<any>('/api/cf/refs?acc=' + acc + '&zid=' + encodeURIComponent(zid) + '&rid=' + encodeURIComponent(rec.id))
      setDlg({
        k: 'list',
        title: '🔗 CNAMEهای وارده',
        rows: (d.refs || []).map((r: any) => ({ title: r.name, sub: r.zone })),
        empty: 'هیچ دامنه‌ای به این رکورد CNAME نکرده است.',
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  async function bulk(op: string, extra: any = {}) {
    if (view.v !== 'zone' || sel.size === 0) return
    const d = await post('/api/cf/bulk', { op, acc: view.acc, zid: view.zid, ids: Array.from(sel), ...extra })
    if (d) {
      setSel(new Set())
      setSelMode(false)
    }
  }

  function bulkEdit() {
    setDlg({
      k: 'form',
      title: '✏️ تغییر مقدار',
      desc: 'مقدار جدید برای همهٔ رکوردهای انتخاب‌شده اعمال می‌شود.',
      fields: [{ key: 'content', label: 'مقدار جدید', placeholder: '1.2.3.4 یا host.example.com' }],
      okText: 'اعمال',
      onSubmit: (v) => bulk('edit', { content: v.content }),
    })
  }
  function bulkType() {
    setDlg({
      k: 'pick',
      title: '🔄 تغییر نوع',
      items: ['A', 'AAAA', 'CNAME'].map((t) => ({ id: t, title: t })),
      onPick: (t) => {
        setDlg(null)
        void bulk('type', { type: t })
      },
    })
  }

  /* ---------------- settings/email helpers ---------------- */

  function setSetting(key: string, value: any) {
    if (view.v !== 'zone') return
    void post('/api/cf/settings', { op: 'set', acc: view.acc, zid: view.zid, key, value })
  }

  function openZoneDetails() {
    if (!zone) return
    setDlg({
      k: 'list',
      title: 'ℹ️ جزئیات دامنه',
      rows: [
        { title: zone.name, sub: 'نام' },
        { title: zone.id, sub: 'شناسه' },
        { title: statusLabel(zone.status), sub: 'وضعیت' },
        { title: zone.plan || '—', sub: 'پلن' },
        { title: zone.created || '—', sub: 'ساخت' },
        { title: zone.account || '—', sub: 'اکانت' },
        ...(zone.ns || []).map((n) => ({ title: n, sub: 'نیم‌سرور' })),
      ],
    })
  }

  function openCatchAll() {
    if (!email) return
    const verified = (email.dests || []).filter((d) => d.verified)
    setDlg({
      k: 'pick',
      title: '📥 مقصد catch-all',
      items: [
        ...verified.map((d) => ({ id: d.email, title: d.email })),
        { id: '__off', title: '⏸ غیرفعال کردن', sub: 'catch-all خاموش شود' },
      ],
      onPick: (id) => {
        setDlg(null)
        const body = id === '__off' ? { op: 'catchall', on: false } : { op: 'catchall', on: true, dest: id }
        if (view.v === 'zone') void post('/api/cf/email', { acc: view.acc, zid: view.zid, ...body })
      },
    })
  }

  /* ---------------- derived ---------------- */

  const rootTitle =
    view.v === 'zones'
      ? 'کلادفلر'
      : view.v === 'accounts'
        ? 'اکانت‌های کلادفلر'
        : view.v === 'favs'
          ? 'منتخب‌ها'
          : view.v === 'search'
            ? 'جست‌وجو'
            : view.v === 'lbzones'
              ? 'لود بالانسر'
              : view.v === 'lb'
                ? 'گروه‌های لود بالانسر'
                : zone
                  ? zone.name
                  : 'دامنه'
  const title =
    view.v === 'zone'
      ? tab === 'settings'
        ? 'تنظیمات · ' + (zone?.name || '')
        : tab === 'email'
          ? 'ایمیل · ' + (zone?.name || '')
          : tab === 'traffic'
            ? 'ترافیک · ' + (zone?.name || '')
            : zone?.name || 'دامنه'
      : view.v === 'lbgroup'
        ? lbName || 'لود بالانسر'
        : rootTitle

  /* ---------------- zones ---------------- */

  function zonesBody(list: CFZone[], emptyText = 'دامنه‌ای پیدا نشد.', onPick?: (z: CFZone) => void) {
    const ql = q.trim().toLowerCase()
    const filtered = list.filter(
      (z) =>
        (accFilter === -1 || z.acc === accFilter) &&
        (!ql || z.name.toLowerCase().includes(ql) || (z.accName || '').toLowerCase().includes(ql)),
    )
    const count = filtered.length
    return (
      <>
        <Input
          className="cc-input cc-mb"
          placeholder="جست‌وجوی دامنه…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <div className="cc-chips">
          <button className={'cc-chip' + (accFilter === -1 ? ' on' : '')} onClick={() => setAccFilter(-1)}>
            همه اکانت‌ها
          </button>
          {accs.map((a) => (
            <button
              key={a.i}
              className={'cc-chip' + (accFilter === a.i ? ' on' : '')}
              onClick={() => setAccFilter(a.i)}
            >
              {a.name}
            </button>
          ))}
        </div>
        <div className="cc-section">{num(count)} دامنه</div>
        {count === 0 && !busy && <Empty title={emptyText} />}
        {filtered.map((z) => (
          <LayerCard
            key={z.id}
            className="cc-card cc-row cc-tap"
            role="button"
            tabIndex={0}
            onClick={() => (onPick ? onPick(z) : push({ v: 'zone', acc: z.acc, zid: z.id }))}
          >
            <span className={'cc-dot ' + (z.status === 'active' ? 'run' : z.status === 'paused' ? 'off' : 'wait')} />
            <div className="cc-grow">
              <div className="cc-name cc-mono">{z.name}</div>
              <div className="cc-sub">{z.accName || 'اکانت ' + (z.acc + 1)}</div>
            </div>
            <Badge variant={ZONE_BADGE[z.status] || 'secondary'}>{statusLabel(z.status)}</Badge>
            <span className="cc-chev">›</span>
          </LayerCard>
        ))}
      </>
    )
  }

  function zonesPage() {
    return (
      <>
        <div className="cc-provider">
          <button className="cc-provider-btn" onClick={onSwitch}>
            هتزنر
          </button>
          <button className="cc-provider-btn on">کلادفلر</button>
        </div>
        <div className="cc-grid2 cc-mb">
          <Button className="cc-green" icon={Star} onClick={() => push({ v: 'favs' })}>
            منتخب‌ها
          </Button>
          <Button variant="primary" onClick={() => push({ v: 'accounts' })}>
            👤 اکانت‌ها
          </Button>
        </div>
        <Button
          className="cc-wide cc-green cc-mb"
          onClick={() => {
            const active = zones.filter((z) => z.status === 'active')
            setDlg({
              k: 'pick',
              title: '➕ افزودن رکورد به کدام دامنه؟',
              items: (active.length ? active : zones).map((z) => ({
                id: z.acc + '|' + z.id,
                title: z.name,
                sub: z.accName,
              })),
              onPick: (id) => {
                const [a, zid] = id.split('|')
                const z = zones.find((x) => x.id === zid)
                setDlg(null)
                setPendingEditor({ acc: Number(a), zid, zone: z?.name || '' })
                push({ v: 'zone', acc: Number(a), zid })
              },
            })
          }}
        >
          ➕ افزودن رکورد
        </Button>
        <div className="cc-grid2 cc-mb">
          <Button variant="outline" onClick={() => push({ v: 'search' })}>
            🔍 جست‌وجوی رکورد
          </Button>
          <Button variant="outline" onClick={() => push({ v: 'lbzones' })}>
            🛟 لود بالانسر
          </Button>
        </div>
        {zonesBody(zones)}
      </>
    )
  }

  /* ---------------- favorites/search ---------------- */

  function openInZone(acc: number, zid: string, zoneName: string, rid: string) {
    if (view.v === 'zone' && view.acc === acc && view.zid === zid) {
      const hit = records.find((r) => r.id === rid)
      if (hit) setDlg({ k: 'record', acc, zid, zone: zoneName, rec: hit })
      return
    }
    setPendingRid(rid)
    push({ v: 'zone', acc, zid })
  }

  function favsPage() {
    return (
      <>
        {favs.length === 0 && !busy && <Empty title="منتخب‌ها خالی است" description="از داخل رکورد، ستاره را بزن." />}
        {favs.map((f) => (
          <LayerCard key={f.rid} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">
                {f.label || f.rid}
                {f.missing && <Badge variant="error">حذف‌شده</Badge>}
              </div>
              <div className="cc-sub">
                {f.zone || ''} {f.content ? ' · ' + f.content : ''}
              </div>
            </div>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                if (f.missing) {
                  void post('/api/cf/favs', { rid: f.rid })
                  return
                }
                openInZone(f.acc, f.zid, f.zone, f.rid)
              }}
            >
              {f.missing ? 'حذف' : 'بازکردن'}
            </Button>
          </LayerCard>
        ))}
      </>
    )
  }

  function searchPage() {
    return (
      <>
        <Input
          autoFocus
          className="cc-input cc-mb"
          placeholder="نام رکورد، آی‌پی یا دامنه…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        {!search && q.trim().length < 2 && <div className="cc-hint cc-center">حداقل ۲ حرف وارد کن.</div>}
        {!search && q.trim().length >= 2 && <div className="cc-center cc-pad"><Loader /></div>}
        {search && (
          <>
            {search.zones.length > 0 && <div className="cc-section">{num(search.zones.length)} دامنه</div>}
            {search.zones.map((z) => (
              <LayerCard key={z.id} className="cc-card cc-row cc-tap" onClick={() => push({ v: 'zone', acc: z.acc, zid: z.id })}>
                <div className="cc-grow">
                  <div className="cc-name cc-mono">{z.name}</div>
                  <div className="cc-sub">{z.accName}</div>
                </div>
                <Badge variant={ZONE_BADGE[z.status] || 'secondary'}>{statusLabel(z.status)}</Badge>
              </LayerCard>
            ))}
            <div className="cc-section">{num(search.records.length)} رکورد</div>
            {search.records.length === 0 && search.zones.length === 0 && <Empty title="چیزی پیدا نشد" />}
            {search.records.map((r) => (
              <LayerCard
                key={r.id}
                className="cc-card cc-row cc-tap"
                onClick={() => openInZone(r.acc, r.zid, r.zone, r.id)}
              >
                <Badge variant="secondary">{r.type}</Badge>
                <div className="cc-grow">
                  <div className="cc-name cc-mono">{relName(r.name, r.zone)}</div>
                  <div className="cc-sub cc-mono">{r.content}</div>
                </div>
                <div className="cc-sub">{r.zone}</div>
              </LayerCard>
            ))}
          </>
        )}
      </>
    )
  }

  /* ---------------- accounts ---------------- */

  function accountsPage() {
    return (
      <>
        {accs.length === 0 && !busy && <Empty title="اکانتی ثبت نشده" description="توکن کلادفلر را اضافه کن." />}
        {accs.map((a) => (
          <LayerCard key={a.i} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">{a.name}</div>
              <div className="cc-sub">{(a.zones ?? 0) >= 0 ? num(a.zones) + ' دامنه' : 'خطا در خواندن دامنه‌ها'}</div>
            </div>
            <Button
              variant="secondary-destructive"
              size="sm"
              onClick={() =>
                setDlg({
                  k: 'confirm',
                  title: 'حذف اکانت',
                  desc: `اکانت «${a.name}» حذف شود؟ دامنه‌ها حذف نمی‌شوند ولی از پنل خارج می‌شوند.`,
                  danger: true,
                  okText: 'حذف',
                  onOk: async () => {
                    await post('/api/cf/accounts', { op: 'del', i: a.i })
                  },
                })
              }
            >
              حذف
            </Button>
          </LayerCard>
        ))}
        <Button
          className="cc-wide cc-mt"
          variant="primary"
          onClick={() =>
            setDlg({
              k: 'form',
              title: '➕ افزودن اکانت کلادفلر',
              desc: 'توکن باید دسترسی Zone → DNS → Edit و Email Routing داشته باشد.',
              fields: [
                { key: 'name', label: 'نام اکانت', placeholder: 'مثلاً main' },
                { key: 'token', label: 'API Token', placeholder: 'توکن کلادفلر', password: true },
              ],
              okText: 'افزودن',
              onSubmit: async (v) => {
                await post('/api/cf/accounts', { op: 'add', name: v.name, token: v.token })
              },
            })
          }
        >
          ➕ افزودن اکانت
        </Button>
      </>
    )
  }

  /* ---------------- zone ---------------- */

  function recordRow(r: CFRec) {
    const checked = sel.has(r.id)
    return (
      <LayerCard
        key={r.id}
        className={'cc-card cc-row cc-tap' + (selMode ? ' cc-selecting' : '')}
        onClick={() => {
          if (view.v !== 'zone') return
          if (selMode) {
            const s = new Set(sel)
            if (checked) s.delete(r.id)
            else s.add(r.id)
            setSel(s)
          } else {
            setDlg({ k: 'record', acc: view.acc, zid: view.zid, zone: zone?.name || '', rec: r })
          }
        }}
      >
        {selMode ? (
          <span className={'cc-check' + (checked ? ' on' : '')}>{checked ? '✓' : ''}</span>
        ) : (
          <Badge variant={PROXYABLE.includes(r.type) && r.proxied ? 'success' : 'secondary'}>{r.type}</Badge>
        )}
        <div className="cc-grow">
          <div className="cc-name cc-mono">
            {relName(r.name, zone?.name || '')}
            {r.fav && <span className="cc-star"> ★</span>}
          </div>
          <div className="cc-sub cc-mono">{r.content}</div>
          {PROXYABLE.includes(r.type) && (
            <div className="cc-sub">
              {r.proxied ? 'پروکسی روشن' : 'پروکسی خاموش'} · TTL {ttlLabel(r.ttl)}
            </div>
          )}
        </div>
        {!selMode && <span className="cc-chev">›</span>}
      </LayerCard>
    )
  }

  function recordsTab() {
    if (view.v !== 'zone') return null
    const ql = q.trim().toLowerCase()
    const list = records.filter(
      (r) =>
        !ql ||
        r.name.toLowerCase().includes(ql) ||
        r.content.toLowerCase().includes(ql) ||
        r.type.toLowerCase() === ql,
    )
    return (
      <>
        <div className="cc-grid2 cc-mb">
          <Button
            variant="primary"
            onClick={() => setDlg({ k: 'editor', acc: view.acc, zid: view.zid, zone: zone?.name || '' })}
          >
            ➕ رکورد
          </Button>
          <Button
            variant={selMode ? 'secondary' : 'outline'}
            onClick={() => {
              setSelMode(!selMode)
              setSel(new Set())
            }}
          >
            {selMode ? '✅ پایان' : '🗂 گروهی'}
          </Button>
        </div>
        {selMode && (
          <div className="cc-bulkbar">
            <button className="cc-bulkbtn red" disabled={!sel.size} onClick={() => bulk('del')}>
              🗑 حذف
            </button>
            <button className="cc-bulkbtn" disabled={!sel.size} onClick={bulkEdit}>
              ✏️ مقدار
            </button>
            <button className="cc-bulkbtn" disabled={!sel.size} onClick={bulkType}>
              🔄 نوع
            </button>
            <button className="cc-bulkbtn green" disabled={!sel.size} onClick={() => bulk('fav')}>
              ⭐ منتخب
            </button>
          </div>
        )}
        <Input className="cc-input cc-mb" placeholder="جست‌وجو در رکوردها…" value={q} onChange={(e) => setQ(e.target.value)} />
        <div className="cc-section">
          {num(list.length)} رکورد{sel.size > 0 ? ' · ' + faNum(sel.size) + ' انتخاب' : ''}
        </div>
        {list.length === 0 && !busy && <Empty title="رکوردی نیست" />}
        {list.map(recordRow)}
      </>
    )
  }

  function settingsTab() {
    if (view.v !== 'zone') return null
    if (!settings) return <div className="cc-center cc-pad"><Loader /></div>
    return (
      <>
        <div className="cc-grid2 cc-mb">
          <Button
            className="cc-green"
            onClick={() =>
              setDlg({
                k: 'confirm',
                title: settings.dev ? 'خاموش کردن حالت توسعه' : 'روشن کردن حالت توسعه',
                desc: 'حالت توسعه کش را دور می‌زند و برای تست مناسب است.',
                onOk: async () => {
                  if (view.v === 'zone') await post('/api/cf/settings', { op: 'dev', acc: view.acc, zid: view.zid })
                },
              })
            }
          >
            🚧 حالت توسعه: {settings.dev ? 'روشن' : 'خاموش'}
          </Button>
          <Button
            variant="secondary-destructive"
            onClick={() =>
              setDlg({
                k: 'confirm',
                title: 'پاک‌سازی کش',
                desc: 'کل کش دامنه پاک شود؟',
                onOk: async () => {
                  if (view.v === 'zone') await post('/api/cf/settings', { op: 'purge', acc: view.acc, zid: view.zid })
                },
              })
            }
          >
            🧹 پاک‌سازی کش
          </Button>
        </div>
        {settings.groups.map((g) => (
          <div key={g.key}>
            <div className="cc-section">{g.title}</div>
            <LayerCard className="cc-card">
              {g.items.map((it) => (
                <div className="cc-kv" key={it.key}>
                  <span>{it.label}</span>
                  {it.kind === 'bool' ? (
                    <Button
                      size="sm"
                      className={it.value === 'on' ? 'cc-green' : ''}
                      variant={it.value === 'on' ? undefined : 'secondary'}
                      onClick={() => setSetting(it.key, it.value === 'on' ? 'off' : 'on')}
                    >
                      {it.value === 'on' ? 'روشن' : 'خاموش'}
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        setDlg({
                          k: 'pick',
                          title: it.label,
                          items: (it.values || []).map((v) => ({
                            id: v.v,
                            title: v.l,
                            sub: v.v === it.value ? 'فعلی' : '',
                          })),
                          onPick: (v) => {
                            setDlg(null)
                            setSetting(it.key, v)
                          },
                        })
                      }
                    >
                      {it.values?.find((v) => v.v === it.value)?.l || it.value} ▾
                    </Button>
                  )}
                </div>
              ))}
            </LayerCard>
          </div>
        ))}
        <div className="cc-grid2 cc-mt">
          <Button variant="outline" onClick={openZoneDetails}>
            ℹ️ جزئیات دامنه
          </Button>
          <Button
            className={settings.paused ? 'cc-green' : ''}
            variant={settings.paused ? undefined : 'secondary-destructive'}
            onClick={() =>
              setDlg({
                k: 'confirm',
                title: settings.paused ? 'فعال‌سازی دامنه' : 'توقف دامنه',
                desc: settings.paused
                  ? 'دامنه دوباره روی کلادفلر فعال شود؟'
                  : 'دامنه متوقف شود؟ سایت از دسترس خارج می‌شود.',
                danger: !settings.paused,
                onOk: async () => {
                  if (view.v === 'zone') await post('/api/cf/settings', { op: 'pause', acc: view.acc, zid: view.zid })
                },
              })
            }
          >
            {settings.paused ? '▶️ فعال‌سازی دامنه' : '⏸ توقف دامنه'}
          </Button>
        </div>
      </>
    )
  }

  function emailTab() {
    if (view.v !== 'zone') return null
    if (!email) return <div className="cc-center cc-pad"><Loader /></div>
    if (!email.available) {
      return (
        <Empty
          title="دسترسی ایمیل نیست"
          description="به توکن دسترسی Zone → Email Routing Rules → Edit اضافه کن."
        />
      )
    }
    if (!email.enabled) {
      return (
        <>
          <LayerCard className="cc-card">
            <div className="cc-name">✉️ ایمیل فوروارد</div>
            <div className="cc-sub">
              با فعال‌سازی، رکوردهای MX/SPF خودکار ساخته می‌شوند. اگر سرویس ایمیل دیگری داری، خراب می‌شود.
            </div>
          </LayerCard>
          <Button
            className="cc-wide cc-green"
            onClick={() =>
              setDlg({
                k: 'confirm',
                title: 'فعال‌سازی ایمیل',
                desc: 'ایمیل روی این دامنه فعال شود؟',
                okText: 'فعال کن',
                onOk: async () => {
                  if (view.v === 'zone') await post('/api/cf/email', { op: 'enable', acc: view.acc, zid: view.zid })
                },
              })
            }
          >
            ✉️ فعال‌سازی ایمیل
          </Button>
        </>
      )
    }
    const verified = (email.dests || []).filter((d) => d.verified)
    return (
      <>
        <div className="cc-section">قوانین فوروارد ({num(email.rules.length)})</div>
        {email.rules.length === 0 && <div className="cc-hint cc-center cc-mb">قانونی نیست.</div>}
        {email.rules.map((r) => (
          <LayerCard key={r.id} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name cc-mono">{r.label}</div>
              <div className="cc-sub cc-mono">→ {r.dest}</div>
            </div>
            <Button
              variant="destructive"
              size="sm"
              onClick={() =>
                setDlg({
                  k: 'confirm',
                  title: 'حذف قانون',
                  desc: `${r.label} → ${r.dest} حذف شود؟`,
                  danger: true,
                  okText: 'حذف',
                  onOk: async () => {
                    if (view.v === 'zone') await post('/api/cf/email', { op: 'del', acc: view.acc, zid: view.zid, rid: r.id })
                  },
                })
              }
            >
              🗑
            </Button>
          </LayerCard>
        ))}
        <Button
          className="cc-wide cc-green cc-mb"
          onClick={() =>
            setDlg({
              k: 'form',
              title: '➕ آدرس جدید',
              desc: 'آدرس محلی (مثلاً info) و مقصد تأییدشده را وارد کن.',
              fields: [
                { key: 'addr', label: 'آدرس', placeholder: 'info' },
                {
                  key: 'dest',
                  label: 'مقصد',
                  select: verified.length
                    ? verified.map((d) => ({ value: d.email, label: d.email }))
                    : undefined,
                  placeholder: 'you@gmail.com',
                },
              ],
              okText: 'ساختن',
              onSubmit: async (v) => {
                if (view.v === 'zone')
                  await post('/api/cf/email', { op: 'add', acc: view.acc, zid: view.zid, addr: v.addr, dest: v.dest })
              },
            })
          }
        >
          ➕ آدرس فوروارد
        </Button>

        <div className="cc-section">📥 catch-all</div>
        <LayerCard className="cc-card cc-row">
          <div className="cc-grow">
            <div className="cc-sub">ایمیل‌های بدون قانون</div>
            <div className="cc-name">{email.catchAll || '—'}</div>
          </div>
          <Button variant="outline" size="sm" onClick={openCatchAll}>
            تنظیم
          </Button>
        </LayerCard>

        <div className="cc-section">📧 مقصدها ({num((email.dests || []).length)})</div>
        {(email.dests || []).map((d) => (
          <LayerCard key={d.email} className="cc-card cc-row">
            <span className={'cc-dot ' + (d.verified ? 'run' : 'wait')} />
            <div className="cc-grow">
              <div className="cc-name cc-mono">{d.email}</div>
              <div className="cc-sub">{d.verified ? 'تأییدشده' : 'در انتظار تأیید'}</div>
            </div>
          </LayerCard>
        ))}
        <Button
          className="cc-wide cc-mt"
          variant="outline"
          onClick={() =>
            setDlg({
              k: 'form',
              title: 'افزودن مقصد',
              desc: 'لینک تأیید به این ایمیل می‌رود.',
              fields: [{ key: 'dest', label: 'ایمیل مقصد', placeholder: 'you@gmail.com' }],
              okText: 'افزودن',
              onSubmit: async (v) => {
                if (view.v === 'zone')
                  await post('/api/cf/email', { op: 'dest_add', acc: view.acc, zid: view.zid, dest: v.dest })
              },
            })
          }
        >
          ➕ مقصد جدید
        </Button>
      </>
    )
  }

  function trafficTab() {
    return (
      <>
        <div className="cc-chips cc-mb">
          {[1, 7, 30].map((d) => (
            <button key={d} className={'cc-chip' + (days === d ? ' on' : '')} onClick={() => setDays(d)}>
              {d === 1 ? 'امروز' : d === 7 ? '۷ روز' : '۳۰ روز'}
            </button>
          ))}
        </div>
        {!traffic ? (
          <div className="cc-center cc-pad"><Loader /></div>
        ) : (
          <>
            <LayerCard className="cc-card">
              <div className="cc-name">📊 مجموع کوئری DNS</div>
              <div className="cc-kv">
                <span>بازه</span>
                <span>{traffic.days === 1 ? 'امروز' : faNum(traffic.days) + ' روز'}</span>
              </div>
              <div className="cc-kv">
                <span>مجموع</span>
                <span>{num(traffic.total)}</span>
              </div>
            </LayerCard>
            <div className="cc-section">پرتکرارترین نام‌ها</div>
            {traffic.top.length === 0 && <div className="cc-hint cc-center">داده‌ای نیست.</div>}
            {traffic.top.map((r) => (
              <LayerCard key={r.name} className="cc-card cc-row">
                <div className="cc-grow">
                  <div className="cc-name cc-mono">{relName(r.name, zone?.name || '')}</div>
                  <div className="cc-sub cc-mono">{r.name}</div>
                </div>
                <span className="cc-sub">{num(r.count)}</span>
              </LayerCard>
            ))}
          </>
        )}
      </>
    )
  }

  function zonePage() {
    return (
      <>
        <LayerCard className="cc-card cc-row">
          <span className={'cc-dot ' + (zone?.status === 'active' ? 'run' : zone?.status === 'paused' ? 'off' : 'wait')} />
          <div className="cc-grow">
            <div className="cc-name cc-mono">{zone?.name || ''}</div>
            <div className="cc-sub">
              {zone?.account || ''} {zone?.plan ? ' · ' + zone.plan : ''}
            </div>
          </div>
          <Badge variant={ZONE_BADGE[zone?.status || ''] || 'secondary'}>{statusLabel(zone?.status || '')}</Badge>
        </LayerCard>
        <div className="cc-tabs">
          {(
            [
              ['records', 'رکوردها'],
              ['settings', 'تنظیمات'],
              ['email', 'ایمیل'],
              ['traffic', 'ترافیک'],
            ] as const
          ).map(([k, l]) => (
            <button
              key={k}
              className={'cc-tab' + (tab === k ? ' on' : '')}
              onClick={() => {
                setTab(k)
                setQ('')
              }}
            >
              {l}
            </button>
          ))}
        </div>
        {tab === 'records' && recordsTab()}
        {tab === 'settings' && settingsTab()}
        {tab === 'email' && emailTab()}
        {tab === 'traffic' && trafficTab()}
      </>
    )
  }

  /* ---------------- load balancer ---------------- */

  function lbZonesPage() {
    return (
      <>
        <div className="cc-hint cc-mb">دامنه‌ای را برای مدیریت گروه‌های لود بالانسر انتخاب کن.</div>
        {zonesBody(zones, 'دامنه‌ای پیدا نشد.', (z) => push({ v: 'lb', acc: z.acc, zid: z.id }))}
      </>
    )
  }

  function lbPage() {
    if (view.v !== 'lb') return null
    return (
      <>
        <div className="cc-section">گروه‌های {zone?.name || ''}</div>
        {lbGroups.length === 0 && !busy && <Empty title="گروهی نیست" description="رکورد A/AAAA/CNAME بساز تا گروه تشکیل شود." />}
        {lbGroups.map((g) => (
          <LayerCard
            key={g.name}
            className="cc-card cc-row cc-tap"
            onClick={() => push({ v: 'lbgroup', acc: view.acc, zid: view.zid, name: g.name })}
          >
            <div className="cc-grow">
              <div className="cc-name cc-mono">{relName(g.name, zone?.name || '')}</div>
              <div className="cc-sub">
                {faNum(g.on)} فعال · {faNum(g.off)} غیرفعال
              </div>
            </div>
            <span className="cc-chev">›</span>
          </LayerCard>
        ))}
      </>
    )
  }

  function lbGroupPage() {
    if (view.v !== 'lbgroup') return null
    function entryAction(op: string, extra: any = {}) {
      if (view.v !== 'lbgroup') return
      void post('/api/cf/lb', { op, acc: view.acc, zid: view.zid, name: view.name, ...extra })
    }
    return (
      <>
        <div className="cc-grid2 cc-mb">
          <Button
            variant="primary"
            onClick={() =>
              setDlg({
                k: 'form',
                title: '➕ افزودن آی‌پی',
                fields: [
                  {
                    key: 'type',
                    label: 'نوع',
                    select: ['A', 'AAAA', 'CNAME'].map((t) => ({ value: t, label: t })),
                  },
                  { key: 'content', label: 'مقدار', placeholder: '1.2.3.4 یا host.example.com' },
                ],
                okText: 'افزودن',
                onSubmit: async (v) => {
                  if (view.v === 'lbgroup') {
                    await post('/api/cf/lb', {
                      op: 'add',
                      acc: view.acc,
                      zid: view.zid,
                      name: view.name,
                      type: v.type,
                      content: v.content,
                    })
                  }
                },
              })
            }
          >
            ➕ افزودن آی‌پی
          </Button>
          <Button variant="outline" onClick={() => entryAction('proxy', { on: true })}>
            🟠 پروکسی همه
          </Button>
        </div>
        <div className="cc-grid2 cc-mb">
          <Button variant="outline" onClick={() => entryAction('proxy', { on: false })}>
            ⚪ پروکسی خاموش
          </Button>
          <Button variant="outline" onClick={() => push({ v: 'lb', acc: view.acc, zid: view.zid })}>
            ↩️ گروه‌ها
          </Button>
        </div>
        <div className="cc-section">{faNum(lbEntries.length)} آی‌پی</div>
        {lbEntries.length === 0 && !busy && <Empty title="آی‌پی‌ای نیست" />}
        {lbEntries.map((e) => (
          <LayerCard key={e.content + e.type} className="cc-card">
            <div className="cc-row">
              <span className={'cc-dot ' + (e.on ? 'run' : 'off')} />
              <div className="cc-grow">
                <div className="cc-name cc-mono">{e.content}</div>
                <div className="cc-sub">
                  {e.type} · وزن {faNum(e.weight)}
                  {e.spare ? ' · زاپاس: ' + e.spare.content : ' · بدون زاپاس'}
                </div>
              </div>
              <Badge variant={e.on ? 'success' : 'neutral'}>{e.on ? 'فعال' : 'غیرفعال'}</Badge>
            </div>
            <div className="cc-grid2 cc-mt4">
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  setDlg({
                    k: 'form',
                    title: '⚖️ وزن آی‌پی',
                    fields: [{ key: 'weight', label: 'وزن (۰ یا بیشتر)', value: String(e.weight) }],
                    okText: 'ذخیره',
                    onSubmit: async (v) => {
                      if (view.v === 'lbgroup')
                        await post('/api/cf/lb', {
                          op: 'weight',
                          acc: view.acc,
                          zid: view.zid,
                          name: view.name,
                          content: e.content,
                          weight: Number(v.weight || 0),
                        })
                    },
                  })
                }
              >
                ⚖️ وزن
              </Button>
              <Button
                variant={e.on ? 'secondary-destructive' : 'primary'}
                size="sm"
                onClick={() => entryAction('toggle', { content: e.content, on: !e.on })}
              >
                {e.on ? '⏸ غیرفعال' : '▶️ فعال'}
              </Button>
              <Button
                className="cc-green"
                size="sm"
                onClick={() =>
                  setDlg({ k: 'spare', acc: view.acc, zid: view.zid, name: view.name, entry: e })
                }
              >
                🛟 زاپاس
              </Button>
              {e.spare && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => entryAction('replace', { content: e.content })}
                >
                  🔄 جایگزینی
                </Button>
              )}
            </div>
          </LayerCard>
        ))}
      </>
    )
  }

  /* ---------------- dialogs ---------------- */

  function renderDlg() {
    if (!dlg) return null
    return (
      <Dialog.Root open onOpenChange={(o: boolean) => !o && setDlg(null)}>
        <Dialog className="cc-dialog" size="base">
          {dlg.k === 'confirm' && (
            <>
              <Dialog.Title className="cc-dialog-title">{dlg.title}</Dialog.Title>
              {dlg.desc && <Dialog.Description className="cc-dialog-desc">{dlg.desc}</Dialog.Description>}
              <div className="cc-dialog-actions">
                <Button variant="secondary" onClick={() => setDlg(null)}>
                  انصراف
                </Button>
                <Button
                  className={dlg.danger ? '' : 'cc-green'}
                  variant={dlg.danger ? 'destructive' : undefined}
                  onClick={() => {
                    const f = dlg.onOk
                    setDlg(null)
                    void f()
                  }}
                >
                  {dlg.okText || 'تأیید'}
                </Button>
              </div>
            </>
          )}
          {dlg.k === 'form' && (
            <FormDialog key={dlg.title + ':' + dlg.fields.map((f) => f.key).join(',')} fields={dlg.fields} title={dlg.title} desc={dlg.desc} okText={dlg.okText} onSubmit={dlg.onSubmit} onClose={() => setDlg(null)} />
          )}
          {dlg.k === 'pick' && (
            <>
              <Dialog.Title className="cc-dialog-title">{dlg.title}</Dialog.Title>
              <div className="cc-picker">
                {dlg.items.length === 0 && <div className="cc-hint cc-center">گزینه‌ای نیست.</div>}
                {dlg.items.map((it) => (
                  <button key={it.id} className="cc-choice" onClick={() => dlg.onPick(it.id)}>
                    <span className="cc-grow">
                      <span className="cc-name">{it.title}</span>
                      {it.sub && <span className="cc-sub">{it.sub}</span>}
                    </span>
                    <span className="cc-chev">›</span>
                  </button>
                ))}
              </div>
              <div className="cc-dialog-actions">
                <Button variant="secondary" onClick={() => setDlg(null)}>
                  انصراف
                </Button>
              </div>
            </>
          )}
          {dlg.k === 'list' && (
            <>
              <Dialog.Title className="cc-dialog-title">{dlg.title}</Dialog.Title>
              <div className="cc-picker">
                {dlg.rows.length === 0 && <div className="cc-hint cc-center">{dlg.empty || 'موردی نیست.'}</div>}
                {dlg.rows.map((r, i) => (
                  <div className="cc-kv" key={i}>
                    <span className="cc-sub">{r.sub || ''}</span>
                    <span className="cc-mono cc-small">{r.title}</span>
                  </div>
                ))}
              </div>
              <div className="cc-dialog-actions">
                <Button variant="secondary" onClick={() => setDlg(null)}>
                  بستن
                </Button>
              </div>
            </>
          )}
          {dlg.k === 'record' && (
            <RecordSheet
              key={dlg.rec.id}
              dlg={dlg}
              onClose={() => setDlg(null)}
              onEdit={() => setDlg({ k: 'editor', acc: dlg.acc, zid: dlg.zid, zone: dlg.zone, rec: dlg.rec })}
              onCopy={copy}
              onFav={() => toggleFav(dlg.acc, dlg.zid, dlg.rec)}
              onProxy={() => toggleProxy(dlg.acc, dlg.zid, dlg.rec)}
              onTTL={() => cycleTTL(dlg.acc, dlg.zid, dlg.rec)}
              onType={() =>
                setDlg({
                  k: 'confirm',
                  title: 'تغییر نوع رکورد',
                  desc: `نوع از ${dlg.rec.type} به ${dlg.rec.type === 'A' ? 'CNAME' : 'A'} تغییر کند؟ مقدار باید با نوع جدید سازگار باشد.`,
                  onOk: async () => {
                    if (view.v === 'zone') {
                      await post('/api/cf/record', {
                        op: 'type',
                        acc: dlg.acc,
                        zid: dlg.zid,
                        rid: dlg.rec.id,
                        type: dlg.rec.type === 'A' ? 'CNAME' : 'A',
                      })
                    }
                  },
                })
              }
              onTraffic={() => showRecordTraffic(dlg.acc, dlg.zid, dlg.rec, dlg.zone)}
              onRefs={() => showRefs(dlg.acc, dlg.zid, dlg.rec)}
              onLB={() => {
                const d = dlg
                setDlg(null)
                push({ v: 'lbgroup', acc: d.acc, zid: d.zid, name: d.rec.name })
              }}
              onDelete={() =>
                setDlg({
                  k: 'confirm',
                  title: 'حذف رکورد',
                  desc: `${dlg.rec.label} برای همیشه حذف می‌شود.`,
                  danger: true,
                  okText: 'حذف',
                  onOk: async () => {
                    if (view.v === 'zone') {
                      await post('/api/cf/record', {
                        op: 'del',
                        acc: dlg.acc,
                        zid: dlg.zid,
                        rid: dlg.rec.id,
                      })
                    }
                  },
                })
              }
            />
          )}
          {dlg.k === 'editor' && (
            <RecordEditor
              key={dlg.rec?.id || 'new'}
              dlg={dlg}
              onClose={() => setDlg(null)}
              onSubmit={async (payload) => {
                const path = '/api/cf/record'
                const body = { ...payload, acc: dlg.acc, zid: dlg.zid, rid: dlg.rec?.id }
                const d = await post(path, body)
                if (d) setDlg(null)
              }}
            />
          )}
          {dlg.k === 'spare' && (
            <SpareSheet
              dlg={dlg}
              onClose={() => setDlg(null)}
              onSet={(spare) => {
                setDlg(null)
                if (view.v === 'lbgroup') {
                  void post('/api/cf/lb', {
                    op: 'spare_set',
                    acc: dlg.acc,
                    zid: dlg.zid,
                    name: dlg.name,
                    content: dlg.entry.content,
                    spare,
                  })
                }
              }}
              onDel={() => {
                setDlg(null)
                if (view.v === 'lbgroup') {
                  void post('/api/cf/lb', {
                    op: 'spare_del',
                    acc: dlg.acc,
                    zid: dlg.zid,
                    name: dlg.name,
                    content: dlg.entry.content,
                  })
                }
              }}
            />
          )}
        </Dialog>
      </Dialog.Root>
    )
  }

  /* ---------------- render ---------------- */

  let body: any = null
  if (err && (view.v === 'zones' || view.v === 'accounts' || view.v === 'favs')) {
    body = <Empty title="خطا" description={err} />
  } else if (view.v === 'zones') body = zonesPage()
  else if (view.v === 'accounts') body = accountsPage()
  else if (view.v === 'favs') body = favsPage()
  else if (view.v === 'search') body = searchPage()
  else if (view.v === 'zone') body = zonePage()
  else if (view.v === 'lbzones') body = lbZonesPage()
  else if (view.v === 'lb') body = lbPage()
  else if (view.v === 'lbgroup') body = lbGroupPage()

  const root = stack.length === 0 && view.v !== 'zones'

  return (
    <div className="cc-app">
      <header className="cc-topbar">
        {(root || stack.length > 0) && (
          <Button variant="ghost" shape="square" icon={ArrowRight} aria-label="بازگشت" onClick={goBack} />
        )}
        <h1 className="cc-title">{title}</h1>
        <Button variant="ghost" shape="square" icon={ArrowClockwise} aria-label="بروزرسانی" onClick={refresh} />
      </header>
      <main className="cc-main">{body}</main>
      {busy > 0 && (
        <div className="cc-loading">
          <Loader size="lg" />
        </div>
      )}
      {toastMsg && <div className={'cc-toast' + (toastMsg.err ? ' err' : '')}>{toastMsg.t}</div>}
      {renderDlg()}
    </div>
  )
}

/* ---------------- record sheet ---------------- */

function RecordSheet({
  dlg,
  onClose,
  onEdit,
  onCopy,
  onFav,
  onProxy,
  onTTL,
  onType,
  onTraffic,
  onRefs,
  onLB,
  onDelete,
}: {
  dlg: Extract<Dlg, { k: 'record' }>
  onClose: () => void
  onEdit: () => void
  onCopy: (v: string, l?: string) => void
  onFav: () => void
  onProxy: () => void
  onTTL: () => void
  onType: () => void
  onTraffic: () => void
  onRefs: () => void
  onLB: () => void
  onDelete: () => void
}) {
  const r = dlg.rec
  return (
    <>
      <Dialog.Title className="cc-dialog-title">
        {r.fav ? '★ ' : ''}
        {r.label}
      </Dialog.Title>
      <Dialog.Description className="cc-dialog-desc">
        <span className="cc-mono">{r.content}</span>
      </Dialog.Description>
      <div className="cc-kv">
        <span>نوع</span>
        <span>{r.type}</span>
      </div>
      <div className="cc-kv">
        <span>TTL</span>
        <span>{ttlLabel(r.ttl)}</span>
      </div>
      {PROXYABLE.includes(r.type) && (
        <div className="cc-kv">
          <span>پروکسی</span>
          <span>{r.proxied ? 'روشن' : 'خاموش'}</span>
        </div>
      )}
      <div className="cc-kv">
        <span>نام کامل</span>
        <span className="cc-mono cc-small">{r.name}</span>
      </div>
      <div className="cc-grid2 cc-mt">
        <Button variant="primary" onClick={onEdit}>
          ✏️ ویرایش مقدار
        </Button>
        <Button className="cc-green" onClick={onFav}>
          {r.fav ? '⭐ حذف از منتخب' : '⭐ افزودن به منتخب'}
        </Button>
        {PROXYABLE.includes(r.type) && (
          <Button className={r.proxied ? '' : 'cc-green'} variant={r.proxied ? 'secondary' : undefined} onClick={onProxy}>
            🌐 پروکسی {r.proxied ? 'خاموش' : 'روشن'}
          </Button>
        )}
        <Button variant="outline" onClick={onTTL}>
          ⏱ TTL: {ttlLabel(r.ttl)}
        </Button>
        {(r.type === 'A' || r.type === 'CNAME') && (
          <Button variant="outline" onClick={onType}>
            🔄 تبدیل به {r.type === 'A' ? 'CNAME' : 'A'}
          </Button>
        )}
        <Button variant="outline" onClick={() => onCopy(r.name, 'نام کپی شد')}>
          📋 کپی نام
        </Button>
        <Button variant="outline" onClick={() => onCopy(r.content, 'مقدار کپی شد')}>
          📋 کپی مقدار
        </Button>
        <Button variant="outline" onClick={onTraffic}>
          📊 ترافیک امروز
        </Button>
        <Button variant="outline" onClick={onRefs}>
          🔗 CNAMEهای وارده
        </Button>
        {PROXYABLE.includes(r.type) && (
          <Button className="cc-green cc-wide" onClick={onLB}>
            ⚙️ لود بالانسر و آی‌پی زاپاس
          </Button>
        )}
        <Button className="cc-wide" variant="destructive" onClick={onDelete}>
          🗑 حذف رکورد
        </Button>
      </div>
      <div className="cc-dialog-actions">
        <Button variant="secondary" onClick={onClose}>
          بستن
        </Button>
      </div>
    </>
  )
}

/* ---------------- record editor ---------------- */

function RecordEditor({
  dlg,
  onClose,
  onSubmit,
}: {
  dlg: Extract<Dlg, { k: 'editor' }>
  onClose: () => void
  onSubmit: (v: any) => Promise<void>
}) {
  const r = dlg.rec
  const [type, setType] = useState(r?.type || 'A')
  const [name, setName] = useState(r ? relName(r.name, dlg.zone) : '')
  const [content, setContent] = useState(r?.content || '')
  const [ttl, setTTL] = useState(r?.ttl || 1)
  const [proxied, setProxied] = useState(!!r?.proxied)
  const [priority, setPriority] = useState(r?.priority != null ? String(r.priority) : '')
  const [busy, setBusy] = useState(false)
  const canProxy = PROXYABLE.includes(type)

  async function submit() {
    if (!content.trim() || (!r && !name.trim())) return
    setBusy(true)
    try {
      await onSubmit({
        op: r ? 'edit' : 'add',
        type,
        name: r ? undefined : name.trim(),
        content: content.trim(),
        ttl: Number(ttl || 1),
        proxied: canProxy && proxied,
        priority: type === 'MX' && priority !== '' ? Number(priority) : undefined,
        force: false,
      })
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <Dialog.Title className="cc-dialog-title">{r ? '✏️ ویرایش رکورد' : '➕ رکورد جدید'}</Dialog.Title>
      {!r && (
        <>
          <div className="cc-section">نوع</div>
          <div className="cc-chips">
            {TYPES.map((t) => (
              <button key={t} className={'cc-chip' + (type === t ? ' on' : '')} onClick={() => setType(t)}>
                {t}
              </button>
            ))}
          </div>
          <div className="cc-section">نام (نسبت به دامنه)</div>
          <Input
            className="cc-input cc-mb"
            placeholder={dlg.zone ? 'مثلاً sub یا @' : 'sub'}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </>
      )}
      <div className="cc-section">مقدار</div>
      <Input
        autoFocus
        className="cc-input cc-mb cc-mono"
        placeholder={
          type === 'CNAME' ? 'host.example.com' : type === 'MX' ? 'mail.example.com' : type === 'TXT' ? 'متن رکورد' : '1.2.3.4'
        }
        value={content}
        onChange={(e) => setContent(e.target.value)}
      />
      {type === 'MX' && (
        <>
          <div className="cc-section">اولویت (Priority)</div>
          <Input className="cc-input cc-mb" placeholder="10" value={priority} onChange={(e) => setPriority(e.target.value)} />
        </>
      )}
      <div className="cc-section">TTL</div>
      <select className="cc-select cc-mb" value={ttl} onChange={(e) => setTTL(Number(e.target.value))}>
        {TTL_STEPS.map((t) => (
          <option key={t} value={t}>
            {ttlLabel(t)}
          </option>
        ))}
      </select>
      {canProxy && (
        <LayerCard className="cc-card cc-row">
          <div className="cc-grow">
            <div className="cc-name">پروکسی کلادفلر</div>
            <div className="cc-sub">ترافیک از شبکهٔ کلادفلر عبور کند.</div>
          </div>
          <Button
            className={proxied ? 'cc-green' : ''}
            variant={proxied ? undefined : 'secondary'}
            size="sm"
            onClick={() => setProxied(!proxied)}
          >
            {proxied ? 'روشن' : 'خاموش'}
          </Button>
        </LayerCard>
      )}
      <div className="cc-dialog-actions">
        <Button variant="secondary" onClick={onClose}>
          انصراف
        </Button>
        <Button className="cc-green" disabled={busy} onClick={() => void submit()}>
          {r ? 'ذخیره' : 'افزودن'}
        </Button>
      </div>
    </>
  )
}

/* ---------------- spare sheet ---------------- */

function SpareSheet({
  dlg,
  onClose,
  onSet,
  onDel,
}: {
  dlg: Extract<Dlg, { k: 'spare' }>
  onClose: () => void
  onSet: (v: string) => void
  onDel: () => void
}) {
  const [v, setV] = useState(dlg.entry.spare?.content || '')
  return (
    <>
      <Dialog.Title className="cc-dialog-title">🛟 آی‌پی زاپاس</Dialog.Title>
      <Dialog.Description className="cc-dialog-desc">
        زاپاس برای <span className="cc-mono">{dlg.entry.content}</span>
      </Dialog.Description>
      {dlg.entry.spare && (
        <div className="cc-kv">
          <span>زاپاس فعلی</span>
          <span className="cc-mono">{dlg.entry.spare.content}</span>
        </div>
      )}
      <div className="cc-section">آی‌پی یا دامنهٔ زاپاس</div>
      <Input className="cc-input cc-mb cc-mono" placeholder="1.2.3.4 یا backup.example.com" value={v} onChange={(e) => setV(e.target.value)} />
      <div className="cc-grid2">
        <Button className="cc-green" disabled={!v.trim()} onClick={() => onSet(v.trim())}>
          💾 ذخیره
        </Button>
        {dlg.entry.spare && (
          <Button variant="destructive" onClick={onDel}>
            🗑 حذف زاپاس
          </Button>
        )}
      </div>
      <div className="cc-dialog-actions">
        <Button variant="secondary" onClick={onClose}>
          بستن
        </Button>
      </div>
    </>
  )
}

/* ---------------- form dialog ---------------- */

function FormDialog({
  fields,
  title,
  desc,
  okText,
  onSubmit,
  onClose,
}: {
  fields: Field[]
  title: string
  desc?: string
  okText?: string
  onSubmit: (v: Record<string, string>) => void | Promise<void>
  onClose: () => void
}) {
  const [vals, setVals] = useState<Record<string, string>>(() => {
    const o: Record<string, string> = {}
    for (const f of fields) o[f.key] = f.value || f.select?.[0]?.value || ''
    return o
  })
  const [busy, setBusy] = useState(false)
  async function go() {
    setBusy(true)
    try {
      await onSubmit(vals)
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      <Dialog.Title className="cc-dialog-title">{title}</Dialog.Title>
      {desc && <Dialog.Description className="cc-dialog-desc">{desc}</Dialog.Description>}
      {fields.map((f) => (
        <div key={f.key}>
          <div className="cc-section">{f.label}</div>
          {f.select ? (
            <select
              className="cc-select cc-mb"
              value={vals[f.key]}
              onChange={(e) => setVals({ ...vals, [f.key]: e.target.value })}
            >
              {f.select.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          ) : (
            <Input
              autoFocus
              type={f.password ? 'password' : 'text'}
              className="cc-input cc-mb"
              placeholder={f.placeholder}
              value={vals[f.key]}
              onChange={(e) => setVals({ ...vals, [f.key]: e.target.value })}
            />
          )}
        </div>
      ))}
      <div className="cc-dialog-actions">
        <Button variant="secondary" onClick={onClose}>
          انصراف
        </Button>
        <Button className="cc-green" disabled={busy} onClick={() => void go()}>
          {okText || 'تأیید'}
        </Button>
      </div>
    </>
  )
}
