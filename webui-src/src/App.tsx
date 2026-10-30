import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { Badge, Button, Dialog, Empty, Input, LayerCard, Loader, Meter } from './components/ui'
import {
  ArrowClockwise,
  ArrowCounterClockwise,
  ArrowRight,
  ArrowUp,
  ArrowsClockwise,
  Camera,
  CaretLeft,
  CheckCircle,
  Copy,
  GlobeHemisphereWest,
  Key,
  List,
  PencilSimple,
  Plus,
  Power,
  Scissors,
  SignIn,
  Trash,
  WarningCircle,
  Wrench,
} from '@phosphor-icons/react'
import { api, ApiError, fallbackCopy, faDate, faNum, gb, gbf, haptic, invalidateCache, mem, num, setToken, TG, token } from './lib'
import { AppVersion } from './components/AppVersion'
import { CopyBtn } from './components/CopyBtn'
import { LogoutButton } from './components/LogoutButton'
import { useSectionTheme } from './components/sectionTheme'
import { WebLogin } from './components/WebLogin'
import { FormDialog, Tools, TOOLS, toolItems } from './components/Tools'
import type { ToolId } from './components/Tools'
import { Sidebar, ThemeToggle } from './components/Sidebar'
import { StatGrid } from './components/Stats'
import { GlobalSearch } from './components/GlobalSearch'
import { FilterBar } from './components/FilterBar'
import type { Provider } from './components/Sidebar'
import type { SubItem } from './components/Sidebar'
import { ProviderShell, ProviderSwitch } from './components/ProviderShell'

const Cloudflare = lazy(() => import('./Cloudflare'))
const Arvan = lazy(() => import('./Arvan'))

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

function addr(s: Server) {
  if (s.ipv4 && s.ipv6) return s.ipv4 + ' / ' + s.ipv6
  return s.ipv4 || s.ipv6 || 'بدون آی‌پی'
}
function badgeVariant(st: string): any {
  if (st === 'running') return 'success'
  if (st === 'off') return 'neutral'
  if (st === 'starting' || st === 'migrating' || st === 'rebuilding') return 'warning'
  if (st === 'unknown' || st === 'deleting') return 'error'
  return 'secondary'
}
/* ---------------- app ---------------- */

export default function App() {
  const [phase, setPhase] = useState<'boot' | 'ready' | 'login' | 'error' | 'signedout'>('boot')
  const [authNonce, setAuthNonce] = useState(0)
  const [authErr, setAuthErr] = useState('')
  const [cur, setCur] = useState<View>({ v: 'accounts' })
  const [navOpen, setNavOpen] = useState(() => window.matchMedia('(min-width: 900px)').matches)
  const [tool, setTool] = useState<ToolId | null>('home')
  // deep links from the bot: /app/?go=srv|rem|pg|hz|cf|arv
  useEffect(() => {
    const go = new URLSearchParams(location.search).get('go')
    if (!go) return
    if (go === 'home' || go === 'srv' || go === 'rem' || go === 'pg' || go === 'um' || go === 'dom' || go === 'hf' || go === 'hfd' || go === 'hook' || go === 'rec' || go === 'mail' || go === 'set') setTool(go)
    else if (go === 'hz' || go === 'cf' || go === 'arv') setProvider(go)
    history.replaceState(null, '', location.pathname)
  }, [])
  const pickProvider = (p: Provider) => {
    setTool(null)
    setProvider(p)
  }
  const [provider, setProvider] = useState<Provider>(() => {
    const p = sessionStorage.getItem('cg_provider')
    return p === 'cf' || p === 'arv' ? p : 'hz'
  })
  const sectionTheme = useSectionTheme(provider)
  const cfBack = useRef<() => void>(() => {})
    const arvBack = useRef<() => void>(() => {})
  const [stack, setStack] = useState<View[]>([])
  const [q, setQ] = useState('')
  const [aflt, setAflt] = useState('all')
  const [sflt, setSflt] = useState('all')
  const [createName, setCreateName] = useState('')
  const [accounts, setAccounts] = useState<Account[]>([])
  const [accForm, setAccForm] = useState<{ name: string; token: string } | null>(null)
  const [accErr, setAccErr] = useState('')
  const [accDel, setAccDel] = useState<Account | null>(null)
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
  const scrollPos = useRef<Record<string, number>>({})

  useEffect(() => {
    sessionStorage.setItem('cg_provider', provider)
    document.documentElement.dataset.provider = provider
  }, [provider])

  function bump(n: number) {
    setBusy((b) => Math.max(0, b + n))
  }
  function toast(t: string, err = false) {
    setToastMsg({ t, err })
    haptic(err ? 'error' : 'success')
    clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToastMsg(null), err ? 3200 : 2100)
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
    const bootTimer = setTimeout(() => {
      if (cancelled) return
      // only a still-booting app is stuck; a login screen is not a failure
      setPhase((p) => {
        if (p !== 'boot') return p
        setAuthErr('اتصال به سرور برقرار نشد. اینترنت یا دسترسی به دامنه را بررسی کن.')
        return 'error'
      })
    }, 12000)
    ;(async () => {
      if (TG) {
        TG.ready()
        TG.expand()
        TG.disableVerticalSwipes && TG.disableVerticalSwipes()
        TG.MainButton && TG.MainButton.hide()
      }
      // the bot links with a one-time ticket: no approval round-trip needed
      const k = new URLSearchParams(location.search).get('k')
      if (k) {
        try {
          const res = await api<{ token: string }>('/api/login/ticket', { k })
          setToken(res.token)
          const u = new URL(location.href)
          u.searchParams.delete('k')
          history.replaceState(null, '', u.pathname + u.search + u.hash)
          if (!cancelled) setPhase('ready')
          return
        } catch {
          // expired or already used: fall through to the normal login
          const u = new URL(location.href)
          u.searchParams.delete('k')
          history.replaceState(null, '', u.pathname + u.search + u.hash)
        }
      }
      if (localStorage.getItem('cg_signed_out') === '1') {
        if (!cancelled) setPhase('signedout')
        return
      }
      if (!TG || !TG.initData) {
        // browser: reuse the same shell, log in with a Telegram approval
        if (token) {
          try {
            await api('/api/me')
            if (!cancelled) setPhase('ready')
            return
          } catch {
            setToken('')
          }
        }
        if (!cancelled) setPhase('login')
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
        setToken(j.data.token)
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
      clearTimeout(bootTimer)
      clearTimeout(toastTimer.current)
    }
  }, [authNonce])

  useEffect(() => {
    const onSignedOut = () => {
      setToken('')
      setPhase('signedout')
    }
    window.addEventListener('cg:signed-out', onSignedOut)
    return () => window.removeEventListener('cg:signed-out', onSignedOut)
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
    if (provider !== 'hz' || modal || cur.v !== 'accounts' || stack.length) TG.BackButton.show()
    else TG.BackButton.hide()
  }, [modal, cur, stack, provider])

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
        bump(-1)
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cur, phase])

  /* ---------------- navigation ---------------- */

  function vkey(v: View) {
    return v.v + ':' + String((v as any).acc || '') + ':' + String((v as any).id || '')
  }
  function push(v: View) {
    scrollPos.current[vkey(cur)] = window.scrollY
    setStack((s) => [...s, cur])
    setCur(v)
    setQ('')
    requestAnimationFrame(() => window.scrollTo(0, 0))
  }
  function goBack() {
    if (tool) {
    return (
      <ProviderShell
        provider={provider}
        title={tool === 'home' ? 'خانه' : (TOOLS.find((t) => t.id === tool) || TOOLS[0]).label}
        busy={false}
        showBack
        onBack={() => setTool(null)}
        onRefresh={() => invalidateCache()}
        onChange={pickProvider}
        toast={null}
        onTool={setTool}
        activeTool={tool}
      >
        <Tools id={tool} onGo={(p, t) => { if (t) setTool(t); else if (p) pickProvider(p) }} />
      </ProviderShell>
    )
  }

  if (provider !== 'hz') {
      if (provider === 'cf') cfBack.current()
      else arvBack.current()
      return
    }
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
      const target = stack[stack.length - 1]
      scrollPos.current[vkey(cur)] = window.scrollY
      setCur(target)
      setStack(stack.slice(0, -1))
      setQ('')
      requestAnimationFrame(() => window.scrollTo(0, scrollPos.current[vkey(target)] || 0))
      return
    }
    if (TG) TG.close()
  }
  backRef.current = goBack

  function reload() {
    invalidateCache()
    setViewErr('')
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
        danger: op === 'delete' || op === 'ip_delete',
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
    if (!/^[a-z0-9][a-z0-9-]{0,62}$/i.test(name)) {
      toast('نام فقط حروف انگلیسی، عدد و خط تیره باشد', true)
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
      ? provider === 'cf'
        ? 'کلادفلر'
        : 'هتزنر'
      : cur.v === 'servers'
        ? 'سرورها' + (accName(cur.acc) ? ' · ' + accName(cur.acc) : '')
        : cur.v === 'server'
          ? 'جزئیات سرور'
          : cur.v === 'ips'
            ? 'آی‌پی‌های تمیز'
            : 'ساخت سرور'

  function body() {
    if (viewErr) {
      return (
        <div className="cc-card cc-error-card" role="alert">
          <div className="cc-error-ico">
            <WarningCircle size={24} />
          </div>
          <div className="cc-name">مشکلی پیش آمد</div>
          <div className="cc-sub">{viewErr}</div>
          <Button variant="primary" icon={ArrowClockwise} onClick={reload}>
            تلاش دوباره
          </Button>
        </div>
      )
    }
    if (cur.v === 'accounts') {
      const aq = q.trim().toLowerCase()
      const list0 = accounts.filter(
        (a) => (!aq || a.name.toLowerCase().includes(aq)) && (aflt === 'has' ? (a.count || 0) > 0 : aflt === 'none' ? (a.count || 0) === 0 : true),
      )
      const known = accounts.some((a) => a.count !== undefined)
      const totalServers = accounts.reduce((n, a) => n + (a.count || 0), 0)
      return (
        <>
          <StatGrid
            items={[
              { label: 'اکانت‌ها', value: faNum(accounts.length) },
              { label: 'سرورها', value: known ? faNum(totalServers) : '—', pct: known ? 100 : 0 },
            ]}
          />
          <div className="cc-pagebar">
            <Button
              variant="primary"
              icon={Plus}
              onClick={() => {
                setAccErr('')
                setAccForm({ name: '', token: '' })
              }}
            >
              افزودن اکانت
            </Button>
          </div>
          <FilterBar
            q={q}
            onQ={setQ}
            placeholder="جستجوی اکانت…"
            filters={[
              { id: 'all', label: 'همه اکانت‌ها' },
              { id: 'has', label: 'دارای سرور' },
              { id: 'none', label: 'بدون سرور' },
            ]}
            value={aflt}
            onValue={setAflt}
            onRefresh={reload}
          />
          {!list0.length && busy > 0 && <AppSkeleton />}
          {!list0.length && busy === 0 && (
            <Empty title="اکانتی پیدا نشد" description={aq ? 'چیزی با این جستجو پیدا نشد.' : 'با دکمهٔ بالا اولین اکانت هتزنر را اضافه کن.'} />
          )}
          {list0.map((a) => (
            <LayerCard
              key={a.i}
              className="cc-trow cc-tap"
              role="button"
              tabIndex={0}
              onClick={() => {
                setServers([])
                setServersAcc(null)
                push({ v: 'servers', acc: a.i })
              }}
            >
              <span className="cc-avatar">{(a.name || 'C').trim().charAt(0)}</span>
              <div className="cc-grow">
                <div className="cc-name">{a.name || 'اکانت ' + (a.i + 1)}</div>
                <div className="cc-sub">{a.count === undefined ? 'برای دیدن سرورها بزن' : num(a.count) + ' سرور'}</div>
              </div>
              <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
              <button
                type="button"
                className="cc-rowbtn danger"
                aria-label={'حذف ' + (a.name || 'اکانت')}
                onClick={(e) => {
                  e.stopPropagation()
                  setAccDel(a)
                }}
              >
                <Trash size={16} />
              </button>
            </LayerCard>
          ))}
          <div className="cc-hint cc-center cc-mt">هر اکانت را انتخاب کن تا سرورها و آی‌پی‌ها را ببینی.</div>
          {accForm && (
            <FormDialog
              title="افزودن اکانت هتزنر"
              sub="توکن API را از کنسول هتزنر بساز (Read & Write)."
              error={accErr}
              onClose={() => setAccForm(null)}
              onSave={async () => {
                setAccErr('')
                try {
                  await api('/api/hz/accounts/add', { name: accForm.name.trim(), token: accForm.token.trim() })
                  invalidateCache()
                  setAccForm(null)
                  reload()
                } catch (e: any) {
                  setAccErr(e.message || 'ذخیره ناموفق')
                }
              }}
            >
              <label className="cc-field">
                <span>نام اکانت</span>
                <Input
                  value={accForm.name}
                  placeholder="مثلاً main"
                  onChange={(e: any) => setAccForm({ ...accForm, name: e.target.value })}
                />
              </label>
              <label className="cc-field">
                <span>توکن API</span>
                <Input
                  value={accForm.token}
                  placeholder="مثل 3f2a…"
                  autoComplete="off"
                  onChange={(e: any) => setAccForm({ ...accForm, token: e.target.value })}
                />
              </label>
            </FormDialog>
          )}
          {accDel && (
            <Dialog.Root open onOpenChange={(o: boolean) => !o && setAccDel(null)}>
              <Dialog className="cc-dialog" size="base">
                <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
                <Dialog.Title className="cc-dialog-title">حذف اکانت</Dialog.Title>
                <Dialog.Description className="cc-dialog-desc">
                  «{accDel.name}» از لیست حذف می‌شود. سرورهای هتزنر دست‌نخورده می‌مانند.
                </Dialog.Description>
                <div className="cc-dialog-actions">
                  <Button variant="secondary" onClick={() => setAccDel(null)}>
                    انصراف
                  </Button>
                  <Button
                    variant="destructive"
                    onClick={async () => {
                      try {
                        await api('/api/hz/accounts/del', { name: accDel.name })
                        invalidateCache()
                        setAccDel(null)
                        reload()
                      } catch (e: any) {
                        toast(e.message || 'حذف ناموفق', true)
                      }
                    }}
                  >
                    حذف
                  </Button>
                </div>
              </Dialog>
            </Dialog.Root>
          )}
        </>
      )
    }

    if (cur.v === 'servers') {
      const ql = q.trim().toLowerCase()
      const list = servers
        .filter((s) => (sflt === 'run' ? s.status === 'running' : sflt === 'off' ? s.status !== 'running' : true))
        .filter((s) => !ql || (s.name + ' ' + s.ipv4 + ' ' + s.ipv6 + ' ' + (s.type && s.type.name)).toLowerCase().includes(ql))
      const running = list.filter((s) => s.status === 'running').length
      return (
        <>
          <StatGrid
            items={[
              { label: 'سرورها', value: faNum(list.length) },
              { label: 'روشن', value: faNum(running), pct: list.length ? (running / list.length) * 100 : 0 },
              { label: 'خاموش', value: faNum(list.length - running), pct: list.length ? ((list.length - running) / list.length) * 100 : 0 },
            ]}
          />
          <FilterBar
            q={q}
            onQ={setQ}
            placeholder="جست‌وجوی نام یا آی‌پی…"
            filters={[
              { id: 'all', label: 'همه سرورها' },
              { id: 'run', label: 'روشن' },
              { id: 'off', label: 'خاموش' },
            ]}
            value={sflt}
            onValue={setSflt}
            onRefresh={reload}
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
          {servers.length === 0 && busy > 0 && <AppSkeleton />}
          {list.length === 0 && busy === 0 && <Empty title="سروری پیدا نشد" />}
          {list.map((s) => (
            <LayerCard
              key={s.id}
              className="cc-trow cc-tap"
              role="button"
              tabIndex={0}
              onClick={() => push({ v: 'server', acc: cur.acc, id: s.id })}
            >
              <div className="cc-grow">
                <div className="cc-name">{s.name}</div>
                <div className="cc-sub">
                  {(s.type && s.type.name) || ''}
                  {s.type ? ` · ${s.type.cores}vCPU / ${mem(s.type.memory)} / ${s.type.disk}GB` : ''}
                  {s.location ? ` · ${s.location.city || s.location.name}` : ''}
                </div>
                <div className="cc-sub cc-mono cc-mt4 cc-copyline">
                  {s.ipv4 && (
                    <>
                      <span className="cc-copyval">{s.ipv4}</span>
                      <CopyBtn value={s.ipv4} onCopy={copy} label="کپی IPv4" size={14} />
                    </>
                  )}
                  {s.ipv6 && (
                    <>
                      {s.ipv4 && <span>/</span>}
                      <span className="cc-copyval">{s.ipv6}</span>
                      <CopyBtn value={s.ipv6} onCopy={copy} label="کپی IPv6" size={14} />
                    </>
                  )}
                  {!s.ipv4 && !s.ipv6 && <span>بدون آی‌پی</span>}
                </div>
              </div>
              <div className="cc-end">
                <Badge variant={badgeVariant(s.status)} dot>
                  {STATUS[s.status] || s.status}
                </Badge>
              </div>
            </LayerCard>
          ))}
        </>
      )
    }

    if (cur.v === 'server') {
      if (!server) return busy > 0 ? <AppSkeleton rows={3} /> : null
      const s = server
      const tr = s.traffic || ({} as Traffic)
      const pct = Math.min(100, Number(tr.used_pct || 0))
      const snaps = s.snapshots || []
      return (
        <>
          <LayerCard className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">{s.name}</div>
              <div className="cc-sub">
                {s.image || ''} · {s.location ? s.location.city || s.location.name : ''} · {s.type ? s.type.id : ''}
              </div>
            </div>
            <Badge variant={badgeVariant(s.status)} dot>
              {STATUS[s.status] || s.status}
            </Badge>
          </LayerCard>

          <LayerCard className="cc-card">
            <div className="cc-kv">
              <span>IPv4</span>
              <span className="cc-mono cc-copyline">
                <span className="cc-copyval">{s.ipv4 || '—'}</span>
                {s.ipv4 && <CopyBtn value={s.ipv4} onCopy={copy} label="کپی IPv4" size={14} />}
              </span>
            </div>
            <div className="cc-kv">
              <span>IPv6</span>
              <span className="cc-mono cc-small cc-copyline">
                <span className="cc-copyval">{s.ipv6 || '—'}</span>
                {s.ipv6 && <CopyBtn value={s.ipv6} onCopy={copy} label="کپی IPv6" size={14} />}
              </span>
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
              <span>{faDate(s.created)}</span>
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
              <Button variant="secondary" icon={Power} onClick={() => runAction('poweroff', { id: s.id })}>
                خاموش
              </Button>
            ) : (
              <Button variant="primary" icon={Power} onClick={() => runAction('poweron', { id: s.id })}>
                روشن
              </Button>
            )}
            <Button variant="secondary" icon={ArrowClockwise} onClick={() => runAction('reboot', { id: s.id })}>
              ری‌بوت
            </Button>
            <Button
              variant="secondary"
              icon={ArrowCounterClockwise}
              onClick={() =>
                runAction('reset', {
                  id: s.id,
                  confirm: true,
                  title: 'ریست سخت',
                  desc: `عملیات «ریست سخت» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              ریست سخت
            </Button>
            <Button
              variant="secondary"
              icon={Key}
              onClick={() =>
                runAction('reset_password', {
                  id: s.id,
                  confirm: true,
                  title: 'ریست رمز روت',
                  desc: `عملیات «ریست رمز روت» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              رمز روت
            </Button>
            <Button
              variant="secondary"
              icon={PencilSimple}
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
              تغییر نام
            </Button>
            <Button
              variant="secondary"
              icon={Camera}
              onClick={() =>
                setModal({
                  kind: 'prompt',
                  title: 'توضیح اسنپ‌شات',
                  placeholder: 'اختیاری',
                  onOk: (v) => void doAction({ op: 'snapshot_create', id: s.id, arg: v }),
                })
              }
            >
              اسنپ‌شات
            </Button>
            <Button variant="outline" icon={Wrench} onClick={() => void rebuildFlow(s.id)}>
              ریبیلد
            </Button>
            <Button variant="outline" icon={ArrowUp} onClick={() => void resizeFlow(s.id)}>
              ارتقا
            </Button>
            <Button variant="outline" icon={GlobeHemisphereWest} onClick={() => void ipAssignFlow(s.id)}>
              اتصال IP
            </Button>
            <Button
              variant="outline"
              icon={ArrowsClockwise}
              onClick={() =>
                runAction('ip_replace', {
                  id: s.id,
                  confirm: true,
                  title: 'جایگزینی IPv4',
                  desc: `سرور «${s.name}» خاموش می‌شود، IPv4 قبلی جدا می‌شود، IPv4 تازه ساخته و متصل می‌شود و بعد سرور روشن می‌شود. ادامه؟`,
                })
              }
            >
              تعویض IPv4
            </Button>
            <Button
              className="cc-wide"
              variant="secondary-destructive"
              icon={Scissors}
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
              حذف IPv4 اختصاصی
            </Button>
            <Button
              className="cc-wide"
              variant="secondary-destructive"
              icon={Scissors}
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
              حذف IPv6 اختصاصی
            </Button>
            <Button
              className="cc-wide"
              variant="destructive"
              icon={Trash}
              onClick={() =>
                runAction('delete', {
                  id: s.id,
                  confirm: true,
                  title: 'حذف سرور',
                  desc: `سرور «${s.name}» برای همیشه حذف می‌شود.`,
                })
              }
            >
              حذف سرور
            </Button>
          </div>

          <div className="cc-section">اسنپ‌شات‌ها ({snaps.length})</div>
          {snaps.length === 0 && <div className="cc-hint cc-center">اسنپ‌شاتی نیست.</div>}
          {snaps.map((sn) => (
            <LayerCard key={sn.id} className="cc-card cc-row">
              <div className="cc-grow">
                <div className="cc-name">{sn.name || sn.id}</div>
                <div className="cc-sub">
                  {gb(sn.size_gb)} · {faDate(sn.created)}
                </div>
              </div>
              <Button
                variant="secondary-destructive"
                size="sm"
                icon={Trash}
                aria-label="حذف اسنپ‌شات"
                onClick={() =>
                  setModal({
                    kind: 'confirm',
                    title: 'حذف اسنپ‌شات',
                    desc: 'این اسنپ‌شات برای همیشه حذف می‌شود.',
                    danger: true,
                    onOk: () => doAction({ op: 'snapshot_delete', id: s.id, arg: sn.id }),
                  })
                }
              />

            </LayerCard>
          ))}
        </>
      )
    }

    if (cur.v === 'ips') {
      return (
        <>
          <div className="cc-grid2 cc-mb">
            <Button variant="outline" icon={ArrowRight} onClick={() => setCur({ v: 'servers', acc: cur.acc })}>
              سرورها
            </Button>
            <Button variant="secondary" icon={ArrowClockwise} onClick={() => setCur((c) => ({ ...c }))}>
              بروزرسانی
            </Button>
          </div>
          {ips.length === 0 && busy > 0 && <AppSkeleton />}
          {ips.length === 0 && busy === 0 && <Empty title="آی‌پی‌ای در استخر نیست" />}
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
                  <div className="cc-name cc-mono cc-copyline">
                    <span>{p.ip}</span>
                    <CopyBtn value={p.ip} onCopy={copy} label="کپی آی‌پی" />
                  </div>
                  <div className="cc-sub">
                    {p.loc || ''}
                    {busyIp ? ' · ' + p.assignee : ' · آزاد'}
                    {c && c.listed && c.listed.length ? ' · ' + c.listed.slice(0, 2).join(', ') : ''}
                  </div>
                </div>
                {badge}
                {!busyIp && (
                  <div className="cc-row-actions">
                    <Button variant="outline" size="sm" onClick={() => void ipFreeFlow(p.id)}>
                      اتصال
                    </Button>
                    <Button
                      variant="secondary-destructive"
                      size="sm"
                      icon={Trash}
                      aria-label="حذف آی‌پی"
                      onClick={() =>
                        runAction('ip_delete', {
                          id: String(p.id),
                          ipId: p.id,
                          confirm: true,
                          title: 'حذف آی‌پی',
                          desc: `آی‌پی «${p.ip}» از حساب هتزنر حذف شود؟ این عمل بازگشت‌پذیر نیست.`,
                        })
                      }
                    />
                  </div>
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
        <div className="cc-sub" style={{ marginTop: 12 }}>
          در حال ورود…
        </div>
      </div>
    )
  }
  if (phase === 'signedout') {
    return (
      <div className="cc-main cc-signout">
        <div className="cc-card cc-error-card" role="status">
          <div className="cc-error-ico">
            <SignIn size={24} />
          </div>
          <div className="cc-name">از حساب خارج شدید</div>
          <div className="cc-sub">برای ورود دوباره، دکمه زیر را بزنید.</div>
          <Button
            variant="primary"
            icon={SignIn}
            onClick={() => {
              try {
                localStorage.removeItem('cg_signed_out')
              } catch {
                /* ignore */
              }
              setToken('')
              setAuthErr('')
              setPhase('boot')
              setAuthNonce((n) => n + 1)
            }}
          >
            ورود دوباره
          </Button>
        </div>
      </div>
    )
  }
  if (phase === 'login') {
    return <WebLogin onDone={() => { setAuthNonce((n) => n + 1); setPhase('ready') }} />
  }
  if (phase === 'error') {
    return (
      <div className="cc-main">
        <div className="cc-card cc-error-card" role="alert">
          <div className="cc-error-ico">
            <WarningCircle size={24} />
          </div>
          <div className="cc-name">دسترسی نداریم</div>
          <div className="cc-sub">{authErr}</div>
          <Button variant="primary" icon={ArrowClockwise} onClick={() => window.location.reload()}>
            تلاش دوباره
          </Button>
        </div>
      </div>
    )
  }

  if (tool) {
    return (
      <ProviderShell
        provider={provider}
        title={tool === 'home' ? 'خانه' : (TOOLS.find((t) => t.id === tool) || TOOLS[0]).label}
        busy={false}
        showBack
        onBack={() => setTool(null)}
        onRefresh={() => invalidateCache()}
        onChange={pickProvider}
        toast={null}
        onTool={setTool}
        activeTool={tool}
      >
        <Tools id={tool} onGo={(p, t) => { if (t) setTool(t); else if (p) pickProvider(p) }} />
      </ProviderShell>
    )
  }

  if (provider !== 'hz') {
    return (
      <Suspense
        fallback={
          <div className="cc-boot">
            <Loader size="lg" />
          </div>
        }
      >
        {provider === 'cf' ? (
          <Cloudflare
            onSwitch={pickProvider}
            onTool={setTool}
            onBackRef={(f) => {
              cfBack.current = f
            }}
          />
        ) : (
          <Arvan
            onSwitch={pickProvider}
            onTool={setTool}
            onBackRef={(f) => {
              arvBack.current = f
            }}
          />
        )}
      </Suspense>
    )
  }

  const showBack = !!modal || cur.v !== 'accounts' || stack.length > 0

  const accId = 'acc' in cur ? cur.acc : null
  const sections: SubItem[] = [
    { id: 'accounts', label: 'اکانت‌ها', active: cur.v === 'accounts', onSelect: () => setCur({ v: 'accounts' }) },
  ]
  if (accId != null) {
    sections.push(
      {
        id: 'servers',
        label: 'سرورها',
        active: cur.v === 'servers' || cur.v === 'server',
        onSelect: () => setCur({ v: 'servers', acc: accId }),
      },
      { id: 'ips', label: 'آی‌پی‌ها', active: cur.v === 'ips', onSelect: () => setCur({ v: 'ips', acc: accId }) },
    )
  }

  return (
    <div className={"cc-app p-" + provider} style={sectionTheme.style}>
      <Sidebar
        provider={provider}
        onChange={pickProvider}
        open={navOpen}
        onToggle={() => setNavOpen((o) => !o)}
        tools={toolItems(tool, setTool)}
        sections={sections}
        providerActive={!tool}
      />
      <div className="cc-content">
        <div className={'cc-progress' + (busy > 0 ? ' on' : '')} aria-hidden="true" />
        <header className="cc-topbar">
          <button type="button" className="cc-menu-btn" aria-label="منو" onClick={() => setNavOpen((o) => !o)}>
            <List size={20} />
          </button>
          {showBack && (
            <Button variant="ghost" shape="square" icon={ArrowRight} aria-label="بازگشت" onClick={goBack} />
          )}
          <h1 className="cc-title">{title}</h1>
          <GlobalSearch
            go={(p, t) => {
              if (t) setTool(t)
              else if (p) pickProvider(p)
            }}
          />
          <AppVersion />
          <ThemeToggle />
          {cur.v !== 'create' && (
            <Button
              className={busy > 0 ? 'cc-spin' : ''}
              variant="ghost"
              shape="square"
              icon={ArrowClockwise}
              aria-label="بروزرسانی"
              onClick={reload}
            />
          )}
          <LogoutButton />
        </header>
        <main className="cc-main" aria-busy={busy > 0}>
          <div className="cc-mobile-only">
            {(cur.v === 'accounts' || cur.v === 'servers') && (
              <ProviderSwitch provider={provider} onChange={pickProvider} />
            )}
          </div>
          {body()}
        </main>
        {toastMsg && (
          <div className={'cc-toast ' + (toastMsg.err ? 'err' : 'ok')} role="status" aria-live="polite">
            {toastMsg.err ? <WarningCircle size={18} /> : <CheckCircle size={18} />}
            <span>{toastMsg.t}</span>
          </div>
        )}
        <ModalHost modal={modal} onClose={() => setModal(null)} onCopy={copy} />
      </div>
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
        <Dialog.Close className="cc-dialog-x" aria-label="بستن">
          ✕
        </Dialog.Close>
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
              <Button variant="secondary" icon={Copy} onClick={() => onCopy(modal.password)}>
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
              <span className="cc-mono cc-copyline">
                <span className="cc-copyval">{modal.ip || '—'}</span>
                {modal.ip && <CopyBtn value={modal.ip} onCopy={onCopy} label="کپی IPv4" size={14} />}
              </span>
            </div>
            <div className="cc-secret cc-mono">{modal.password}</div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" icon={Copy} onClick={() => onCopy(`${modal.ip} root ${modal.password}`)}>
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


function AppSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, i) => (
        <LayerCard key={i} className="cc-card cc-skel" aria-hidden="true">
          <span className="cc-skel-line" style={{ width: 40 + ((i * 11) % 24) + '%' }} />
          <span className="cc-skel-line" style={{ width: 64 + ((i * 9) % 18) + '%', height: 8 }} />
        </LayerCard>
      ))}
    </>
  )
}
