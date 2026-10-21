import { useEffect, useRef, useState } from 'react'
import { Badge, Button, Dialog, Empty, Input, LayerCard, Loader, Meter } from '@cloudflare/kumo'
import { ArrowClockwise, ArrowRight, GlobeHemisphereWest, Plus } from '@phosphor-icons/react'

/* ---------------- types ---------------- */

type Account = { i: number; name: string; count?: number }
type SrvType = { id: string; name: string; cores: number; memory: number; disk: number; architecture: string }
type Traffic = { in_gb: number; out_gb: number; total_gb: number; included_gb: number; used_pct: number; extra_gb: number }
type Snapshot = { id: string; name: string; size_gb: number; created: string; description: string }
type Server = {
  id: string
  name: string
  status: string
  ipv4: string
  ipv6: string
  locked: boolean
  created: string
  type: SrvType
  location: { name: string; city: string; country: string }
  image: string
  traffic: Traffic
  snapshots?: Snapshot[]
  prices?: { monthly?: string; hourly?: string }
}
type CleanIP = { id: number; ip: string; loc: string; assignee?: string; auto?: boolean; check?: { clean: boolean; listed: string[]; unknown: boolean } }
type OptLocation = { name: string; city: string; country: string; description: string }
type OptType = SrvType & { monthly?: string }
type OptImage = { id: string; name: string; architecture: string; os_flavor?: string }
type Options = { locations: OptLocation[]; types: OptType[]; images: OptImage[] }

type CreateState = {
  v: 'create'
  acc: number
  step: number
  loc: string
  type: string
  typeName: string
  arch: string
  image: string
  imageName: string
}
type View =
  | { v: 'accounts' }
  | { v: 'servers'; acc: number }
  | { v: 'server'; acc: number; id: string }
  | { v: 'ips'; acc: number }
  | CreateState

type PickerItem = { id: string; title: string; sub?: string; extra?: string }
type Modal =
  | { kind: 'confirm'; title: string; desc?: string; danger?: boolean; okText?: string; onOk: () => void | Promise<void> }
  | { kind: 'prompt'; title: string; placeholder?: string; value?: string; onOk: (v: string) => void | Promise<void> }
  | { kind: 'picker'; title: string; items: PickerItem[]; onPick: (id: string) => void }
  | { kind: 'password'; password: string }
  | { kind: 'created'; name: string; ip: string; password: string; onDone: () => void }

/* ---------------- telegram + api ---------------- */

const TG: any = (window as any).Telegram && (window as any).Telegram.WebApp ? (window as any).Telegram.WebApp : null

let token = sessionStorage.getItem('cg_token') || ''

class ApiError extends Error {}

async function api<T = any>(path: string, body?: any): Promise<T> {
  const headers: Record<string, string> = { Authorization: 'Bearer ' + token }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const res = await fetch(path, {
    method: body === undefined ? 'GET' : 'POST',
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  let j: any = null
  try {
    j = await res.json()
  } catch {
    /* ignore */
  }
  if (res.status === 401) {
    sessionStorage.removeItem('cg_token')
    token = ''
  }
  if (!res.ok || !j || j.ok === false) throw new ApiError((j && j.error) || 'خطای سرور (' + res.status + ')')
  return j.data as T
}

/* ---------------- helpers ---------------- */

const STATUS: Record<string, string> = {
  running: 'روشن',
  off: 'خاموش',
  starting: 'در حال روشن شدن',
  stopping: 'در حال خاموش شدن',
  rebuilding: 'ریبیلد',
  migrating: 'انتقال',
  deleting: 'در حال حذف',
  unknown: 'نامشخص',
}

function num(v: any) {
  return Number(v || 0).toLocaleString('en-US')
}
function gb(v: any) {
  const n = Number(v || 0)
  return n >= 1000 ? (n / 1000).toFixed(1) + ' TB' : n.toFixed(n < 10 ? 1 : 0) + ' GB'
}
function mem(mb: any) {
  const n = Number(mb || 0)
  return n >= 1024 ? (n / 1024).toFixed(n % 1024 ? 1 : 0) + ' GB' : n + ' MB'
}
function faNum(v: any) {
  return Number(v || 0).toLocaleString('fa-IR', { maximumFractionDigits: 1 })
}
function gbf(v: any) {
  const n = Number(v || 0)
  return n >= 1000 ? faNum(n / 1000) + ' ترابایت' : faNum(n) + ' گیگابایت'
}
function addr(s: Server) {
  if (s.ipv4 && s.ipv6) return s.ipv4 + ' / ' + s.ipv6
  return s.ipv4 || s.ipv6 || 'بدون آی‌پی'
}
function dotClass(st: string) {
  if (st === 'running') return 'run'
  if (st === 'off') return 'off'
  if (st === 'starting' || st === 'migrating' || st === 'rebuilding') return 'wait'
  if (st === 'unknown') return 'err'
  return ''
}
function badgeVariant(st: string): any {
  if (st === 'running') return 'success'
  if (st === 'off') return 'neutral'
  if (st === 'starting' || st === 'migrating' || st === 'rebuilding') return 'warning'
  if (st === 'unknown' || st === 'deleting') return 'error'
  return 'secondary'
}
function fallbackCopy(v: string) {
  const ta = document.createElement('textarea')
  ta.value = v
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
  } catch {
    /* ignore */
  }
  ta.remove()
}

/* ---------------- app ---------------- */

export default function App() {
  const [phase, setPhase] = useState<'boot' | 'ready' | 'error'>('boot')
  const [authErr, setAuthErr] = useState('')
  const [cur, setCur] = useState<View>({ v: 'accounts' })
  const [stack, setStack] = useState<View[]>([])
  const [q, setQ] = useState('')
  const [createName, setCreateName] = useState('')
  const [accounts, setAccounts] = useState<Account[]>([])
  const [servers, setServers] = useState<Server[]>([])
  const [serversAcc, setServersAcc] = useState<number | null>(null)
  const [server, setServer] = useState<Server | null>(null)
  const [ips, setIps] = useState<CleanIP[]>([])
  const [opts, setOpts] = useState<Options | null>(null)
  const [viewErr, setViewErr] = useState('')
  const [busy, setBusy] = useState(0)
  const [modal, setModal] = useState<Modal | null>(null)
  const [toastMsg, setToastMsg] = useState<{ t: string; err?: boolean } | null>(null)
  const optsCache = useRef<Record<string, Options>>({})
  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  function bump(n: number) {
    setBusy((b) => Math.max(0, b + n))
  }
  function toast(t: string, err = false) {
    setToastMsg({ t, err })
    clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToastMsg(null), 3200)
  }
  async function copy(v: string) {
    try {
      if (navigator.clipboard) await navigator.clipboard.writeText(v)
      else fallbackCopy(v)
      toast('کپی شد')
    } catch {
      fallbackCopy(v)
      toast('کپی شد')
    }
  }

  /* boot + auth */
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      if (TG) {
        TG.ready()
        TG.expand()
        TG.disableVerticalSwipes && TG.disableVerticalSwipes()
        TG.MainButton && TG.MainButton.hide()
        document.documentElement.dataset.mode = TG.colorScheme === 'light' ? 'light' : 'dark'
      }
      if (!TG || !TG.initData) {
        if (!cancelled) {
          setAuthErr('این مینی‌اپ فقط از داخل ربات تلگرام کار می‌کند.')
          setPhase('error')
        }
        return
      }
      try {
        if (token) {
          try {
            await api('/api/me')
            if (!cancelled) setPhase('ready')
            return
          } catch {
            /* re-auth */
          }
        }
        const res = await fetch('/api/auth', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ initData: TG.initData }),
        })
        const j = await res.json()
        if (!res.ok || !j.ok) throw new ApiError(j.error || 'ورود ناموفق')
        token = j.data.token
        sessionStorage.setItem('cg_token', token)
        if (!cancelled) setPhase('ready')
      } catch (e: any) {
        if (!cancelled) {
          setAuthErr(e.message || 'ورود ناموفق')
          setPhase('error')
        }
      }
    })()
    return () => {
      cancelled = true
      clearTimeout(toastTimer.current)
    }
  }, [])

  /* telegram back button */
  const backRef = useRef<() => void>(() => {})
  useEffect(() => {
    if (!TG || !TG.BackButton) return
    const h = () => backRef.current()
    TG.BackButton.onClick(h)
    return () => TG.BackButton.offClick && TG.BackButton.offClick(h)
  }, [])
  useEffect(() => {
    if (!TG || !TG.BackButton) return
    if (modal || cur.v !== 'accounts' || stack.length) TG.BackButton.show()
    else TG.BackButton.hide()
  }, [modal, cur, stack])

  /* load current view */
  useEffect(() => {
    if (phase !== 'ready') return
    let cancelled = false
    setViewErr('')
    bump(1)
    ;(async () => {
      try {
        if (cur.v === 'accounts') {
          const d = await api<{ accounts: Account[] }>('/api/hz/accounts')
          if (!cancelled) setAccounts(d.accounts || [])
        } else if (cur.v === 'servers') {
          if (serversAcc !== cur.acc) {
            if (!cancelled) {
              setServers([])
              setServersAcc(cur.acc)
            }
          }
          const d = await api<{ servers: Server[] }>('/api/hz/servers?acc=' + cur.acc)
          if (!cancelled) setServers(d.servers || [])
        } else if (cur.v === 'server') {
          const d = await api<{ server: Server }>(
            '/api/hz/server?acc=' + cur.acc + '&id=' + encodeURIComponent(cur.id),
          )
          if (!cancelled) setServer(d.server)
        } else if (cur.v === 'ips') {
          const d = await api<{ ips: CleanIP[] }>('/api/hz/ips?acc=' + cur.acc)
          if (!cancelled) setIps(d.ips || [])
        } else if (cur.v === 'create' && cur.step <= 3) {
          if (!cancelled) setOpts(null)
          const loc = cur.step >= 2 ? cur.loc : ''
          const arch = cur.step >= 3 ? cur.arch : ''
          const key = cur.acc + '|' + loc + '|' + arch
          let o = optsCache.current[key]
          if (!o) {
            o = await api<Options>(
              '/api/hz/options?acc=' + cur.acc +
                (loc ? '&loc=' + encodeURIComponent(loc) : '') +
                (arch ? '&arch=' + encodeURIComponent(arch) : ''),
            )
            optsCache.current[key] = o
          }
          if (!cancelled) setOpts(o)
        }
      } catch (e: any) {
        if (!cancelled) {
          setViewErr(e.message || 'خطا')
          toast(e.message || 'خطا', true)
        }
      } finally {
        if (!cancelled) bump(-1)
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cur, phase])

  /* ---------------- navigation ---------------- */

  function push(v: View) {
    setStack((s) => [...s, cur])
    setCur(v)
    setQ('')
  }
  function goBack() {
    if (modal) {
      setModal(null)
      return
    }
    if (cur.v === 'create' && cur.step > 1) {
      const step = cur.step - 1
      setCur({
        ...cur,
        step,
        ...(step === 1
          ? { loc: '', type: '', typeName: '', arch: '', image: '', imageName: '' }
          : step === 2
            ? { type: '', typeName: '', arch: '', image: '', imageName: '' }
            : { image: '', imageName: '' }),
      })
      return
    }
    if (stack.length) {
      setCur(stack[stack.length - 1])
      setStack(stack.slice(0, -1))
      setQ('')
      return
    }
    if (TG) TG.close()
  }
  backRef.current = goBack

  function reload() {
    if (cur.v === 'create') {
      setOpts(null)
      optsCache.current = {}
    }
    setCur((c) => ({ ...c }))
  }

  /* ---------------- actions ---------------- */

  async function getOptions(acc: number, loc: string, arch: string) {
    const key = acc + '|' + loc + '|' + arch
    let o = optsCache.current[key]
    if (!o) {
      o = await api<Options>(
        '/api/hz/options?acc=' + acc +
          (loc ? '&loc=' + encodeURIComponent(loc) : '') +
          (arch ? '&arch=' + encodeURIComponent(arch) : ''),
      )
      optsCache.current[key] = o
    }
    return o
  }

  async function doAction(o: { op: string; id: string; arg?: string; ipId?: number }) {
    const acc = (cur as any).acc as number
    bump(1)
    try {
      const d = await api<any>('/api/hz/action', {
        acc,
        id: o.id,
        action: o.op,
        arg: o.arg || '',
        ipId: Number(o.ipId || 0),
        confirm: true,
      })
      if (d && d.password) setModal({ kind: 'password', password: d.password })
      else toast((d && d.message) || 'انجام شد')
      setServers([])
      setServersAcc(null)
      if (o.op === 'delete') {
        setStack([])
        setCur({ v: 'servers', acc })
        return
      }
      setCur((c) => ({ ...c }))
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  function runAction(op: string, o?: { id?: string; arg?: string; ipId?: number; confirm?: boolean; title?: string; desc?: string }) {
    const id = o?.id ?? (cur.v === 'server' ? cur.id : '')
    if (o?.confirm) {
      setModal({
        kind: 'confirm',
        title: o.title || 'تأیید عملیات',
        desc: o.desc,
        danger: op === 'delete',
        onOk: () => doAction({ op, id, arg: o.arg, ipId: o.ipId }),
      })
    } else {
      void doAction({ op, id, arg: o?.arg, ipId: o?.ipId })
    }
  }

  async function rebuildFlow(id: string) {
    if (!server) return
    bump(1)
    try {
      const o = await getOptions((cur as any).acc, server.location.name, server.type.architecture)
      setModal({
        kind: 'picker',
        title: 'انتخاب تصویر برای ریبیلد',
        items: o.images.map((im) => ({ id: im.id, title: im.name, sub: im.architecture })),
        onPick: (img) =>
          setModal({
            kind: 'confirm',
            title: 'ریبیلد سرور',
            desc: 'دیسک سرور پاک و تصویر تازه نصب می‌شود. ادامه؟',
            danger: true,
            onOk: () => doAction({ op: 'rebuild', id, arg: img }),
          }),
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  async function resizeFlow(id: string) {
    if (!server) return
    bump(1)
    try {
      const o = await getOptions((cur as any).acc, server.location.name, '')
      const types = o.types.filter((t) => t.architecture === server.type.architecture)
      setModal({
        kind: 'picker',
        title: 'ارتقای پلن (سرور باید خاموش باشد)',
        items: types.map((t) => ({
          id: t.id,
          title: t.name,
          sub: `${t.cores} vCPU · ${mem(t.memory)} · ${t.disk}GB`,
          extra: t.monthly || '',
        })),
        onPick: (tid) =>
          setModal({
            kind: 'confirm',
            title: 'ارتقای پلن',
            desc: 'نوع سرور تغییر می‌کند و دیسک هم ارتقا می‌یابد. ادامه؟',
            onOk: () => doAction({ op: 'resize', id, arg: tid }),
          }),
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  async function ipAssignFlow(id: string) {
    bump(1)
    try {
      const d = await api<{ ips: CleanIP[] }>('/api/hz/ips?acc=' + (cur as any).acc)
      const free = (d.ips || []).filter((p) => !p.assignee)
      setModal({
        kind: 'picker',
        title: 'اتصال آی‌پی (سرور خاموش می‌شود)',
        items: free.map((p) => ({
          id: String(p.id),
          title: p.ip,
          sub: (p.loc || '') + (p.check ? (p.check.clean ? ' · تمیز' : ' · لیست‌شده') : ''),
        })),
        onPick: (ipid) =>
          setModal({
            kind: 'confirm',
            title: 'اتصال آی‌پی',
            desc: 'سرور خاموش، آی‌پی وصل و دوباره روشن می‌شود. ادامه؟',
            onOk: () => doAction({ op: 'ip_assign', id, ipId: Number(ipid) }),
          }),
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  async function ipFreeFlow(ipid: number) {
    bump(1)
    try {
      let list = servers
      if (!list.length) {
        const d = await api<{ servers: Server[] }>('/api/hz/servers?acc=' + (cur as any).acc)
        list = d.servers || []
        setServers(list)
        setServersAcc((cur as any).acc)
      }
      setModal({
        kind: 'picker',
        title: 'اتصال به کدام سرور؟',
        items: list.map((s) => ({ id: s.id, title: s.name, sub: addr(s) })),
        onPick: (sid) =>
          setModal({
            kind: 'confirm',
            title: 'اتصال آی‌پی',
            desc: 'سرور خاموش، آی‌پی وصل و دوباره روشن می‌شود. ادامه؟',
            onOk: () => doAction({ op: 'ip_assign', id: sid, ipId: ipid }),
          }),
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  function submitCreate() {
    if (cur.v !== 'create') return
    const name = createName.trim()
    if (!name) {
      toast('نام سرور را وارد کن', true)
      return
    }
    const st = cur
    setModal({
      kind: 'confirm',
      title: 'ساخت سرور',
      desc: `«${name}» با پلن ${st.type} در ${st.loc} ساخته شود؟`,
      onOk: async () => {
        bump(1)
        try {
          const d = await api<any>('/api/hz/create', {
            acc: st.acc,
            name,
            type: st.type,
            image: st.image,
            location: st.loc,
          })
          setServers([])
          setServersAcc(null)
          setModal({
            kind: 'created',
            name,
            ip: d.ip || '',
            password: d.password || '',
            onDone: () => {
              setStack([])
              setCur({ v: 'servers', acc: st.acc })
              setCreateName('')
            },
          })
        } catch (e: any) {
          toast(e.message || 'خطا', true)
        } finally {
          bump(-1)
        }
      },
    })
  }

  /* ---------------- render helpers ---------------- */

  const accName = (i: number) => {
    const a = accounts.find((x) => x.i === i)
    return a ? a.name : ''
  }
  const title =
    cur.v === 'accounts'
      ? 'هتزنر'
      : cur.v === 'servers'
        ? 'سرورها' + (accName(cur.acc) ? ' · ' + accName(cur.acc) : '')
        : cur.v === 'server'
          ? 'جزئیات سرور'
          : cur.v === 'ips'
            ? 'آی‌پی‌های تمیز'
            : 'ساخت سرور'

  function body() {
    if (viewErr) return <Empty title="خطا" description={viewErr} />
    if (cur.v === 'accounts') {
      if (!accounts.length) return <Empty title="اکانتی ثبت نشده" description="برای افزودن اکانت هتزنر از ربات استفاده کن." />
      return (
        <>
          {accounts.map((a) => (
            <LayerCard
              key={a.i}
              className="cc-card cc-row cc-tap"
              role="button"
              tabIndex={0}
              onClick={() => {
                setServers([])
                setServersAcc(null)
                push({ v: 'servers', acc: a.i })
              }}
            >
              <span className="cc-dot run" />
              <div className="cc-grow">
                <div className="cc-name">{a.name || 'اکانت ' + (a.i + 1)}</div>
                <div className="cc-sub">{a.count === undefined ? 'برای دیدن سرورها بزن' : num(a.count) + ' سرور'}</div>
              </div>
              <span className="cc-chev">›</span>
            </LayerCard>
          ))}
          <div className="cc-hint cc-center cc-mt">هر اکانت را انتخاب کن تا سرورها و آی‌پی‌ها را ببینی.</div>
        </>
      )
    }

    if (cur.v === 'servers') {
      const ql = q.trim().toLowerCase()
      const list = servers.filter(
        (s) => !ql || (s.name + ' ' + s.ipv4 + ' ' + s.ipv6 + ' ' + (s.type && s.type.name)).toLowerCase().includes(ql),
      )
      return (
        <>
          <Input
            className="cc-input cc-mb"
            placeholder="جست‌وجوی نام یا آی‌پی…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <div className="cc-grid2 cc-mb">
            <Button variant="outline" icon={GlobeHemisphereWest} onClick={() => push({ v: 'ips', acc: cur.acc })}>
              آی‌پی‌های تمیز
            </Button>
            <Button
              variant="primary"
              icon={Plus}
              onClick={() => {
                setCreateName('')
                push({ v: 'create', acc: cur.acc, step: 1, loc: '', type: '', typeName: '', arch: '', image: '', imageName: '' })
              }}
            >
              ساخت سرور
            </Button>
          </div>
          <div className="cc-section">{num(list.length)} سرور</div>
          {list.length === 0 && <Empty title="سروری پیدا نشد" />}
          {list.map((s) => (
            <LayerCard
              key={s.id}
              className="cc-card cc-row cc-tap"
              role="button"
              tabIndex={0}
              onClick={() => push({ v: 'server', acc: cur.acc, id: s.id })}
            >
              <span className={'cc-dot ' + dotClass(s.status)} />
              <div className="cc-grow">
                <div className="cc-name">{s.name}</div>
                <div className="cc-sub">
                  {(s.type && s.type.name) || ''}
                  {s.type ? ` · ${s.type.cores}vCPU / ${mem(s.type.memory)} / ${s.type.disk}GB` : ''}
                </div>
                <div className="cc-sub cc-mono cc-mt4">{addr(s)}</div>
              </div>
              <div className="cc-end">
                <Badge variant={badgeVariant(s.status)}>{STATUS[s.status] || s.status}</Badge>
                <div className="cc-sub cc-mt4">{s.location ? s.location.city || s.location.name : ''}</div>
              </div>
            </LayerCard>
          ))}
        </>
      )
    }

    if (cur.v === 'server') {
      if (!server) return null
      const s = server
      const tr = s.traffic || ({} as Traffic)
      const pct = Math.min(100, Number(tr.used_pct || 0))
      const snaps = s.snapshots || []
      return (
        <>
          <LayerCard className="cc-card cc-row">
            <span className={'cc-dot ' + dotClass(s.status)} />
            <div className="cc-grow">
              <div className="cc-name">{s.name}</div>
              <div className="cc-sub">
                {s.image || ''} · {s.location ? s.location.city || s.location.name : ''} · {s.type ? s.type.id : ''}
              </div>
            </div>
            <Badge variant={badgeVariant(s.status)}>{STATUS[s.status] || s.status}</Badge>
          </LayerCard>

          <LayerCard className="cc-card">
            <div className="cc-kv">
              <span>IPv4</span>
              <button className="cc-link cc-mono" onClick={() => copy(s.ipv4)}>
                {s.ipv4 || '—'}
              </button>
            </div>
            <div className="cc-kv">
              <span>IPv6</span>
              <span className="cc-mono cc-small">{s.ipv6 || '—'}</span>
            </div>
            <div className="cc-kv">
              <span>پلن</span>
              <span>
                {s.type ? `${s.type.name} · ${s.type.cores}vCPU / ${mem(s.type.memory)} / ${s.type.disk}GB` : '—'}
              </span>
            </div>
            <div className="cc-kv">
              <span>لوکیشن</span>
              <span>{s.location ? `${s.location.city || ''}، ${s.location.country || ''}` : '—'}</span>
            </div>
            <div className="cc-kv">
              <span>ساخت</span>
              <span>{(s.created || '').slice(0, 10) || '—'}</span>
            </div>
            <div className="cc-kv">
              <span>هزینه</span>
              <span>{s.prices && s.prices.monthly ? s.prices.monthly + ' ماهانه' : '—'}</span>
            </div>
          </LayerCard>

          <LayerCard className="cc-card">
            <Meter label="ترافیک مصرفی" value={pct} max={100} showValue={false} />
            <div className="cc-sub cc-mt4">
              {gbf(tr.total_gb)} از {gbf(tr.included_gb)} · {faNum(pct)}٪ مصرف
              {tr.extra_gb > 0 ? ` · ${gbf(tr.extra_gb)} اضافه` : ''}
            </div>
          </LayerCard>

          <div className="cc-actions">
            {s.status === 'running' ? (
              <Button variant="secondary" onClick={() => runAction('poweroff', { id: s.id })}>
                ⏻ خاموش
              </Button>
            ) : (
              <Button variant="primary" onClick={() => runAction('poweron', { id: s.id })}>
                ⏻ روشن
              </Button>
            )}
            <Button variant="secondary" onClick={() => runAction('reboot', { id: s.id })}>
              ↻ ری‌بوت
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                runAction('reset', {
                  id: s.id,
                  confirm: true,
                  title: 'ریست سخت',
                  desc: `عملیات «ریست سخت» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              ⟳ ریست سخت
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                runAction('reset_password', {
                  id: s.id,
                  confirm: true,
                  title: 'ریست رمز روت',
                  desc: `عملیات «ریست رمز روت» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              🔑 رمز روت
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                setModal({
                  kind: 'prompt',
                  title: 'نام جدید سرور',
                  placeholder: 'مثلاً de-2',
                  value: s.name,
                  onOk: (v) => {
                    if (!v) {
                      toast('نام خالی است', true)
                      return
                    }
                    void doAction({ op: 'rename', id: s.id, arg: v })
                  },
                })
              }
            >
              ✎ تغییر نام
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                setModal({
                  kind: 'prompt',
                  title: 'توضیح اسنپ‌شات',
                  placeholder: 'اختیاری',
                  onOk: (v) => void doAction({ op: 'snapshot_create', id: s.id, arg: v }),
                })
              }
            >
              📸 اسنپ‌شات
            </Button>
            <Button variant="outline" onClick={() => void rebuildFlow(s.id)}>
              💿 ریبیلد
            </Button>
            <Button variant="outline" onClick={() => void resizeFlow(s.id)}>
              ⬆ ارتقا
            </Button>
            <Button variant="outline" onClick={() => void ipAssignFlow(s.id)}>
              🌐 اتصال IP
            </Button>
            <Button
              variant="outline"
              onClick={() =>
                runAction('ip_replace', {
                  id: s.id,
                  confirm: true,
                  title: 'جایگزینی IPv4',
                  desc: `عملیات «جایگزینی IPv4» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              ♻ تعویض IPv4
            </Button>
            <Button
              className="cc-wide"
              variant="secondary-destructive"
              onClick={() =>
                runAction('ip_unassign', {
                  id: s.id,
                  arg: 'ipv4',
                  confirm: true,
                  title: 'حذف IPv4 اختصاصی',
                  desc: `عملیات «حذف IPv4 اختصاصی» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              ✂ حذف IPv4 اختصاصی
            </Button>
            <Button
              className="cc-wide"
              variant="secondary-destructive"
              onClick={() =>
                runAction('ip_unassign', {
                  id: s.id,
                  arg: 'ipv6',
                  confirm: true,
                  title: 'حذف IPv6 اختصاصی',
                  desc: `عملیات «حذف IPv6 اختصاصی» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              ✂ حذف IPv6 اختصاصی
            </Button>
            <Button
              className="cc-wide"
              variant="destructive"
              onClick={() =>
                runAction('delete', {
                  id: s.id,
                  confirm: true,
                  title: 'حذف سرور',
                  desc: `سرور «${s.name}» برای همیشه حذف می‌شود.`,
                })
              }
            >
              🗑 حذف سرور
            </Button>
          </div>

          <div className="cc-section">اسنپ‌شات‌ها ({snaps.length})</div>
          {snaps.length === 0 && <div className="cc-hint cc-center">اسنپ‌شاتی نیست.</div>}
          {snaps.map((sn) => (
            <LayerCard key={sn.id} className="cc-card cc-row">
              <div className="cc-grow">
                <div className="cc-name">{sn.name || sn.id}</div>
                <div className="cc-sub">
                  {gb(sn.size_gb)} · {(sn.created || '').slice(0, 10)}
                </div>
              </div>
              <Button
                variant="secondary-destructive"
                size="sm"
                onClick={() =>
                  setModal({
                    kind: 'confirm',
                    title: 'حذف اسنپ‌شات',
                    desc: 'این اسنپ‌شات برای همیشه حذف می‌شود.',
                    danger: true,
                    onOk: () => doAction({ op: 'snapshot_delete', id: s.id, arg: sn.id }),
                  })
                }
              >
                حذف
              </Button>
            </LayerCard>
          ))}
        </>
      )
    }

    if (cur.v === 'ips') {
      return (
        <>
          <div className="cc-grid2 cc-mb">
            <Button variant="outline" onClick={() => setCur({ v: 'servers', acc: cur.acc })}>
              → سرورها
            </Button>
            <Button variant="secondary" icon={ArrowClockwise} onClick={() => setCur((c) => ({ ...c }))}>
              بروزرسانی
            </Button>
          </div>
          {ips.length === 0 && <Empty title="آی‌پی‌ای در استخر نیست" />}
          {ips.map((p) => {
            const c = p.check
            const badge = !c ? (
              <Badge variant="secondary">بررسی نشده</Badge>
            ) : c.clean ? (
              <Badge variant="success">تمیز</Badge>
            ) : c.listed && c.listed.length ? (
              <Badge variant="error">لیست‌شده</Badge>
            ) : (
              <Badge variant="warning">نامشخص</Badge>
            )
            const busyIp = !!p.assignee
            return (
              <LayerCard key={p.id} className="cc-card cc-row">
                <div className="cc-grow">
                  <div className="cc-name cc-mono">{p.ip}</div>
                  <div className="cc-sub">
                    {p.loc || ''}
                    {busyIp ? ' · ' + p.assignee : ' · آزاد'}
                    {c && c.listed && c.listed.length ? ' · ' + c.listed.slice(0, 2).join(', ') : ''}
                  </div>
                </div>
                {badge}
                {!busyIp && (
                  <Button variant="outline" size="sm" onClick={() => void ipFreeFlow(p.id)}>
                    اتصال
                  </Button>
                )}
              </LayerCard>
            )
          })}
        </>
      )
    }

    /* create wizard */
    const c = cur
    const steps = (
      <div className="cc-steps">
        {[1, 2, 3, 4].map((n) => (
          <i key={n} className={c.step >= n ? 'on' : ''} />
        ))}
      </div>
    )
    if (c.step < 4 && !opts) return <>{steps}<div className="cc-center cc-pad"><Loader /></div></>
    if (c.step === 1 && opts) {
      return (
        <>
          {steps}
          <div className="cc-section">لوکیشن</div>
          {opts.locations.map((l) => (
            <button key={l.name} className="cc-choice" onClick={() => setCur({ ...c, step: 2, loc: l.name })}>
              <span className="cc-grow">
                <span className="cc-name">{l.city || l.name}</span>
                <span className="cc-sub">
                  {l.country || ''} · {l.name}
                </span>
              </span>
            </button>
          ))}
        </>
      )
    }
    if (c.step === 2 && opts) {
      return (
        <>
          {steps}
          <div className="cc-section">پلن · {c.loc}</div>
          {opts.types.length === 0 && <Empty title="پلنی برای این لوکیشن نیست" />}
          {opts.types.map((t) => (
            <button
              key={t.id}
              className="cc-choice"
              onClick={() => setCur({ ...c, step: 3, type: t.id, typeName: t.name, arch: t.architecture })}
            >
              <span className="cc-grow">
                <span className="cc-name">{t.name}</span>
                <span className="cc-sub">
                  {t.cores} vCPU · {mem(t.memory)} RAM · {t.disk}GB · {t.architecture}
                </span>
              </span>
              <span className="cc-sub">{t.monthly || ''}</span>
            </button>
          ))}
        </>
      )
    }
    if (c.step === 3 && opts) {
      return (
        <>
          {steps}
          <div className="cc-section">سیستم‌عامل</div>
          {opts.images.map((im) => (
            <button key={im.id} className="cc-choice" onClick={() => setCur({ ...c, step: 4, image: im.id, imageName: im.name })}>
              <span className="cc-grow">
                <span className="cc-name">{im.name}</span>
                <span className="cc-sub">
                  {im.architecture} · {im.os_flavor || ''}
                </span>
              </span>
            </button>
          ))}
        </>
      )
    }
    return (
      <>
        {steps}
        <div className="cc-section">نام و تأیید</div>
        <Input
          className="cc-input cc-mb"
          placeholder="نام سرور (مثلاً de-1)"
          value={createName}
          maxLength={63}
          onChange={(e) => setCreateName(e.target.value)}
        />
        <LayerCard className="cc-card">
          <div className="cc-kv">
            <span>لوکیشن</span>
            <span>{c.loc}</span>
          </div>
          <div className="cc-kv">
            <span>پلن</span>
            <span>{c.typeName || c.type}</span>
          </div>
          <div className="cc-kv">
            <span>تصویر</span>
            <span className="cc-mono">{c.imageName || c.image}</span>
          </div>
          <div className="cc-kv">
            <span>سیستم</span>
            <span>Ubuntu + رمز روت تصادفی</span>
          </div>
        </LayerCard>
        <Button className="cc-wide cc-submit" variant="primary" icon={Plus} onClick={submitCreate}>
          ساخت سرور
        </Button>
      </>
    )
  }

  if (phase === 'boot') {
    return (
      <div className="cc-boot">
        <Loader size="lg" />
      </div>
    )
  }
  if (phase === 'error') {
    return (
      <div className="cc-main">
        <Empty title="دسترسی نداریم" description={authErr} />
      </div>
    )
  }

  const showBack = !!modal || cur.v !== 'accounts' || stack.length > 0

  return (
    <div className="cc-app">
      <header className="cc-topbar">
        {showBack && (
          <Button variant="ghost" shape="square" icon={ArrowRight} aria-label="بازگشت" onClick={goBack} />
        )}
        <h1 className="cc-title">{title}</h1>
        {cur.v !== 'create' && (
          <Button variant="ghost" shape="square" icon={ArrowClockwise} aria-label="بروزرسانی" onClick={reload} />
        )}
      </header>
      <main className="cc-main">{body()}</main>
      {busy > 0 && (
        <div className="cc-loading">
          <Loader size="lg" />
        </div>
      )}
      {toastMsg && <div className={'cc-toast' + (toastMsg.err ? ' err' : '')}>{toastMsg.t}</div>}
      <ModalHost modal={modal} onClose={() => setModal(null)} onCopy={copy} />
    </div>
  )
}

/* ---------------- modal ---------------- */

function ModalHost({ modal, onClose, onCopy }: { modal: Modal | null; onClose: () => void; onCopy: (v: string) => void }) {
  const [promptVal, setPromptVal] = useState('')
  useEffect(() => {
    if (modal && modal.kind === 'prompt') setPromptVal(modal.value || '')
  }, [modal])
  if (!modal) return null
  return (
    <Dialog.Root open onOpenChange={(o: boolean) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        {modal.kind === 'confirm' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title}</Dialog.Title>
            {modal.desc && <Dialog.Description className="cc-dialog-desc">{modal.desc}</Dialog.Description>}
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={onClose}>
                انصراف
              </Button>
              <Button
                variant={modal.danger ? 'destructive' : 'primary'}
                onClick={() => {
                  const f = modal.onOk
                  onClose()
                  void f()
                }}
              >
                {modal.okText || 'تأیید'}
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'prompt' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title}</Dialog.Title>
            <Input
              autoFocus
              className="cc-input cc-mb"
              placeholder={modal.placeholder}
              value={promptVal}
              onChange={(e) => setPromptVal(e.target.value)}
            />
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={onClose}>
                انصراف
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  const f = modal.onOk
                  const v = promptVal.trim()
                  onClose()
                  void f(v)
                }}
              >
                تأیید
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'picker' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title}</Dialog.Title>
            <div className="cc-picker">
              {modal.items.length === 0 && <div className="cc-hint cc-center">گزینه‌ای نیست.</div>}
              {modal.items.map((it) => (
                <button key={it.id} className="cc-choice" onClick={() => modal.onPick(it.id)}>
                  <span className="cc-grow">
                    <span className="cc-name">{it.title}</span>
                    {it.sub && <span className="cc-sub">{it.sub}</span>}
                  </span>
                  {it.extra && <span className="cc-sub">{it.extra}</span>}
                </button>
              ))}
            </div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={onClose}>
                انصراف
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'password' && (
          <>
            <Dialog.Title className="cc-dialog-title">رمز روت جدید</Dialog.Title>
            <div className="cc-secret cc-mono">{modal.password}</div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => onCopy(modal.password)}>
                کپی
              </Button>
              <Button variant="primary" onClick={onClose}>
                باشه
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'created' && (
          <>
            <Dialog.Title className="cc-dialog-title">سرور ساخته شد</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              این اطلاعات را ذخیره کن؛ رمز روت فقط یک‌بار نشان داده می‌شود.
            </Dialog.Description>
            <div className="cc-kv">
              <span>نام</span>
              <span>{modal.name}</span>
            </div>
            <div className="cc-kv">
              <span>IPv4</span>
              <span className="cc-mono">{modal.ip || '—'}</span>
            </div>
            <div className="cc-secret cc-mono">{modal.password}</div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => onCopy(`${modal.ip} root ${modal.password}`)}>
                کپی همه
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  const f = modal.onDone
                  onClose()
                  f()
                }}
              >
                باشه
              </Button>
            </div>
          </>
        )}
      </Dialog>
    </Dialog.Root>
  )
}
