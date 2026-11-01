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
  Lightning,
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
import { CopyBtn } from './components/CopyBtn'
import { LogoutButton } from './components/LogoutButton'
import { useSectionTheme } from './components/sectionTheme'
import { WebLogin } from './components/WebLogin'
import { FormDialog, TOOLS, toolItems } from './components/toolsMeta'
import type { ToolId } from './components/toolsMeta'
import { Sidebar, ThemeToggle } from './components/Sidebar'
import { StatGrid } from './components/Stats'
import { GlobalSearch } from './components/GlobalSearch'
import { FilterBar } from './components/FilterBar'
import type { Provider } from './components/Sidebar'
import type { SubItem } from './components/Sidebar'
import { ProviderShell, ProviderSwitch } from './components/ProviderShell'
import { AccountsView, ServersView } from './components/HetznerViews'
import { ServerDetail } from './components/HetznerServerView'
import { IpsView } from './components/HetznerIpsView'
import { CreateView } from './components/HetznerCreateView'
import type { Account, CleanIP, OptType, Options, Server, Traffic } from './components/hetzner/types'
import { badgeVariant, locFa, STATUS } from './components/hetzner/ui'
import { AppSkeleton, ModalHost } from './components/AppDialogs'
import type { Modal, PickerItem } from './components/AppDialogs'

const Cloudflare = lazy(() => import('./Cloudflare'))
const Arvan = lazy(() => import('./Arvan'))
const Tools = lazy(() => import('./components/Tools').then((m) => ({ default: m.Tools })))

/* ---------------- types ---------------- */


type PlanGroup = { id: string; title: string; items: OptType[] }

// Same split Hetzner uses on its own server-type page.
function planGroups(types: OptType[]): PlanGroup[] {
  const shared = (t: OptType) => (t.cpu_type || 'shared') !== 'dedicated'
  const defs: { id: string; title: string; test: (t: OptType) => boolean }[] = [
    { id: 'x86', title: 'Shared vCPU · x86', test: (t) => shared(t) && t.architecture !== 'arm' },
    { id: 'arm', title: 'Shared vCPU · Arm64', test: (t) => shared(t) && t.architecture === 'arm' },
    { id: 'ded', title: 'Dedicated vCPU', test: (t) => !shared(t) },
  ]
  const out: PlanGroup[] = []
  const used = new Set<string>()
  for (const d of defs) {
    const items = types.filter((t) => !used.has(t.id) && d.test(t))
    items.forEach((t) => {
      used.add(t.id)
    })
    if (items.length) out.push({ id: d.id, title: d.title, items })
  }
  const rest = types.filter((t) => !used.has(t.id))
  if (rest.length) out.push({ id: 'other', title: 'سایر', items: rest })
  return out
}

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


/* ---------------- helpers ---------------- */


function addr(s: Server) {
  if (s.ipv4 && s.ipv6) return s.ipv4 + ' / ' + s.ipv6
  return s.ipv4 || s.ipv6 || 'بدون آی‌پی'
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
    if (go === 'home' || go === 'srv' || go === 'rem' || go === 'pg' || go === 'um' || go === 'dom' || go === 'hf' || go === 'hfd' || go === 'hook' || go === 'mail' || go === 'set' || go === 'fo' || go === 'hub' || go === 'cfcheck' || go === 'hs') setTool(go)
    else if (go === 'hz' || go === 'cf' || go === 'arv') {
      setTool(null)
      setProvider(go)
    }
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
  const [ipForm, setIpForm] = useState<{ loc: string; count: number; locs: { name: string; city?: string; country?: string }[] } | null>(null)
  const [ipBusy, setIpBusy] = useState(false)
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
    let bootTimer = 0
    const armBoot = () => {
      clearTimeout(bootTimer)
      bootTimer = window.setTimeout(() => {
        if (cancelled) return
        // a phone that slept through the boot is not a failed boot
        if (document.visibilityState === 'hidden') return
        // only a still-booting app is stuck; a login screen is not a failure
        setPhase((p) => {
          if (p !== 'boot') return p
          setAuthErr('اتصال به سرور برقرار نشد. اینترنت یا دسترسی به دامنه را بررسی کن.')
          return 'error'
        })
      }, 12000)
    }
    const onVisible = () => {
      if (!document.hidden) armBoot()
    }
    document.addEventListener('visibilitychange', onVisible)
    armBoot()
    ;(async () => {
      if (TG) {
        TG.ready()
        TG.expand()
        TG.isVersionAtLeast && TG.isVersionAtLeast('7.0') && TG.disableVerticalSwipes && TG.disableVerticalSwipes()
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
      document.removeEventListener('visibilitychange', onVisible)
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
    if (!TG || !TG.BackButton || !TG.isVersionAtLeast || !TG.isVersionAtLeast('6.1')) return
    const h = () => backRef.current()
    TG.BackButton.onClick(h)
    return () => TG.BackButton.offClick && TG.BackButton.offClick(h)
  }, [])
  useEffect(() => {
    if (!TG || !TG.BackButton || !TG.isVersionAtLeast || !TG.isVersionAtLeast('6.1')) return
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
          sub: `${t.cores} vCPU · ${t.memory} GB · ${t.disk} GB`,
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

  async function runCreate(st: CreateState, name: string, typeID: string, imageID: string) {
    bump(1)
    try {
      const d = await api<any>('/api/hz/create', { acc: st.acc, name, type: typeID, image: imageID, location: st.loc })
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
  }

  // «ساخت فوری»: Ubuntu 24.04, default name = plan + two random digits.
  function quickCreate(t: OptType) {
    if (cur.v !== 'create') return
    const st = cur
    const img = (opts?.images || []).find(
      (i) =>
        (!i.architecture || i.architecture === t.architecture) &&
        (/ubuntu-?24\.04/i.test(i.name) || (i.os_flavor === 'ubuntu' && /^24\.04/.test((i as any).os_version || ''))),
    )
    if (!img) {
      toast('Ubuntu 24.04 برای این پلن پیدا نشد؛ از ساخت سفارشی استفاده کن', true)
      return
    }
    const name = t.name + '-' + (Math.floor(Math.random() * 90) + 10)
    setModal({
      kind: 'confirm',
      title: 'ساخت فوری',
      desc: `«${name}» با پلن ${t.name} و Ubuntu 24.04 در ${locFa(st.loc)} ساخته شود؟`,
      onOk: () => runCreate(st, name, t.id, img.id),
    })
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
      onOk: () => runCreate(st, name, st.type, st.image),
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

  async function openIpAdd() {
    const acc = (cur as any).acc
    setIpBusy(true)
    try {
      const o = await api<Options>('/api/hz/options?acc=' + acc)
      const locs = (o.locations || []).map((l: any) => ({ name: l.name, city: l.city, country: l.country }))
      setIpForm({ loc: locs[0]?.name || '', count: 1, locs })
    } catch (e: any) {
      toast(e.message || 'دریافت لوکیشن‌ها ناموفق', true)
    }
    setIpBusy(false)
  }

  async function submitIpAdd() {
    if (!ipForm) return
    setIpBusy(true)
    try {
      const r = await api<{ created: string[] }>('/api/hz/ips/add', {
        acc: (cur as any).acc,
        loc: ipForm.loc,
        count: ipForm.count,
      })
      invalidateCache()
      setIpForm(null)
      toast(`${r.created.length} آی‌پی ساخته شد`)
      setCur((c) => ({ ...c }))
    } catch (e: any) {
      toast(e.message || 'ساخت آی‌پی ناموفق', true)
    }
    setIpBusy(false)
  }

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
    if (cur.v === 'accounts' || cur.v === 'servers') {
      if (cur.v === 'accounts')
        return (
          <AccountsView
            accounts={accounts}
            q={q}
            setQ={setQ}
            aflt={aflt}
            setAflt={setAflt}
            reload={reload}
            busy={busy}
            accForm={accForm}
            setAccForm={setAccForm}
            accErr={accErr}
            setAccErr={setAccErr}
            accDel={accDel}
            setAccDel={setAccDel}
            push={push}
            toast={toast}
            setServers={setServers}
            setServersAcc={setServersAcc}
          />
        )
      return (
        <ServersView
          cur={cur}
          servers={servers}
          q={q}
          setQ={setQ}
          sflt={sflt}
          setSflt={setSflt}
          reload={reload}
          busy={busy}
          push={push}
          copy={copy}
          setCreateName={setCreateName}
        />
      )
    }

    if (cur.v === 'server') {
      return (
        <ServerDetail
          server={server}
          busy={busy}
          runAction={runAction}
          doAction={doAction}
          setModal={setModal}
          toast={toast}
          resizeFlow={resizeFlow}
          rebuildFlow={rebuildFlow}
          ipAssignFlow={ipAssignFlow}
          copy={copy}
        />
      )
    }

    if (cur.v === 'ips') {
      return (
        <IpsView
          ips={ips}
          busy={busy}
          ipBusy={ipBusy}
          ipForm={ipForm}
          setIpForm={setIpForm}
          submitIpAdd={submitIpAdd}
          openIpAdd={openIpAdd}
          ipFreeFlow={ipFreeFlow}
          runAction={runAction}
          copy={copy}
          setCur={setCur}
          acc={cur.acc}
        />
      )
    }

    return (
      <CreateView
        cur={cur}
        opts={opts}
        setCur={setCur}
        createName={createName}
        setCreateName={setCreateName}
        submitCreate={submitCreate}
        planGroups={planGroups}
        quickCreate={quickCreate}
      />
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
          <h1 className="cc-title">{title}</h1>
          {!tool && (cur.v === 'server' || cur.v === 'ips' || cur.v === 'create') && (
            <span className="cc-crumb">
              {cur.v === 'create'
                ? 'ساخت سرور'
                : cur.v === 'ips'
                  ? 'آی‌پی‌های تمیز'
                  : [
                      accounts.find((a) => a.i === (cur as any).acc)?.name,
                      servers.find((x) => x.id === ((cur as any).sid || (cur as any).id))?.name,
                    ]
                      .filter(Boolean)
                      .join(' / ')}
            </span>
          )}
          <GlobalSearch
            go={(p, t) => {
              if (t) setTool(t)
              else if (p) pickProvider(p)
            }}
          />
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
          <button type="button" className="cc-menu-btn" aria-label="منو" onClick={() => setNavOpen((o) => !o)}>
            <List size={20} />
          </button>
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

