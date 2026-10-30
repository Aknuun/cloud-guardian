import { useEffect, useRef, useState } from 'react'
import {
  ArrowClockwise,
  ArrowCounterClockwise,
  ArrowRight,
  Camera,
  CaretLeft,
  Cloud,
  Cube,
  Globe,
  Key,
  MagnifyingGlass,
  TrendUp,
  PencilSimple,
  Plus,
  Power,
  Trash,
  WarningCircle,
} from '@phosphor-icons/react'
import { Badge, Button, Dialog, Empty, Input, LayerCard, Loader } from './components/ui'
import { CopyBtn } from './components/CopyBtn'
import { ProviderShell } from './components/ProviderShell'
import { ActionModalHost, type ActionModal } from './components/ActionModal'
import type { Provider, SubItem } from './components/Sidebar'
import type { ToolId } from './components/Tools'
import { api, fallbackCopy, faDate, faNum, haptic, num, TG } from './lib'

type Account = { i: number; name: string; domains?: number; servers?: number }
type Domain = { id: string; name: string; domain: string; status: string }
type Record_ = { id: string; type: string; name: string; value: string; ttl: number; cloud: boolean }
type Server = {
  id: string
  name: string
  status: string
  status_fa: string
  task: string
  flavor: string
  image: string
  os: string
  cpu: number
  ram: number
  disk: number
  region: string
  region_name: string
  pub: string[]
  priv: string[]
  created: string
  ha: boolean
}
type Snapshot = { id: string; name: string; size: number; status: string; created: string }
type Region = { id: string; name: string }
type Size = { id: string; name: string; cpu: number; ram: number; disk: number; price: number; price_text: string }
type Image = { id: string; name: string }
type Firewall = { id: string; name: string }
type Options = { regions: Region[]; sizes: Size[]; images: Image[]; firewalls: Firewall[] }
type Tab = 'dns' | 'cloud'
type View =
  | { v: 'accounts' }
  | { v: 'dns' }
  | { v: 'records'; domain: string }
  | { v: 'servers' }
  | { v: 'server'; id: string; region: string }
  | { v: 'create'; step: number; region: string; size: string; sizeName: string; image: string; imageName: string; firewall: string }

const RECORD_TYPES = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'SRV']

function statusVariant(s: string): 'success' | 'neutral' | 'warning' | 'error' | 'secondary' {
  const x = s.toLowerCase()
  if (x.includes('run') || x.includes('active')) return 'success'
  if (x.includes('off') || x.includes('stop') || x.includes('shut')) return 'neutral'
  if (x.includes('build') || x.includes('creat') || x.includes('pending') || x.includes('reboot')) return 'warning'
  if (x.includes('error') || x.includes('fail')) return 'error'
  return 'secondary'
}

function isOn(s: string) {
  const x = s.toLowerCase()
  return (x.includes('run') || x.includes('active')) && !x.includes('off')
}

function recHost(name: string, domain: string) {
  const n = (name || '').trim().replace(/\.$/, '')
  if (!n || n === '@') return domain
  if (n === domain || n.endsWith('.' + domain)) return n
  return n + '.' + domain
}

export default function Arvan({
  onSwitch,
  onBackRef,
  onTool,
}: {
  onSwitch: (p: Provider) => void
  onBackRef?: (f: () => void) => void
  onTool?: (id: ToolId) => void
}) {
  const [acc, setAcc] = useState<number | null>(null)
  const [tab, setTab] = useState<Tab>('dns')
  const [view, setView] = useState<View>({ v: 'accounts' })
  const [stack, setStack] = useState<View[]>([])
  const [accounts, setAccounts] = useState<Account[]>([])
  const [domains, setDomains] = useState<Domain[]>([])
  const [records, setRecords] = useState<Record_[]>([])
  const [servers, setServers] = useState<Server[]>([])
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [opts, setOpts] = useState<Options | null>(null)
  const [q, setQ] = useState('')
  const [recQ, setRecQ] = useState('')
  const [recType, setRecType] = useState('all')
  const [regionFilter, setRegionFilter] = useState('all')
  const [createName, setCreateName] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(0)
  const [modal, setModal] = useState<ActionModal | null>(null)
  const [toastMsg, setToastMsg] = useState<{ t: string; err?: boolean } | null>(null)
  const optsCache = useRef<Record<string, Options>>({})
  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const scrollPos = useRef<Record<string, number>>({})

  const [accForm, setAccForm] = useState<{ name: string; token: string } | null>(null)
  const [accBusy, setAccBusy] = useState(false)
  const [accErr, setAccErr] = useState('')
  const [accDel, setAccDel] = useState<Account | null>(null)
  const [arvIP, setArvIP] = useState<{ ip: string; clean: boolean; verdict: string } | null>(null)
  const [arvIPBusy, setArvIPBusy] = useState(false)
  const [arvPlans, setArvPlans] = useState<{ id: string; name: string; cpu: number; ram: number; disk: number; price: string; current: boolean }[] | null>(null)
  const [arvHunt, setArvHunt] = useState<any>(null)
  const [arvHuntOn, setArvHuntOn] = useState(false)

  async function submitAcc() {
    if (!accForm) return
    setAccBusy(true)
    setAccErr('')
    try {
      await api('/api/arv/accounts/add', { name: accForm.name, token: accForm.token })
      setAccForm(null)
      reload()
    } catch (e: any) {
      setAccErr(String(e?.message || e))
    } finally {
      setAccBusy(false)
    }
  }

  async function removeAcc() {
    if (!accDel) return
    try {
      await api('/api/arv/accounts/del', { i: accDel.i })
      setAccDel(null)
      reload()
    } catch (e: any) {
      toast(String(e?.message || e), true)
      setAccDel(null)
    }
  }

  function bump(n: number) {
    setBusy((b) => Math.max(0, b + n))
  }
  function toast(t: string, isErr = false) {
    setToastMsg({ t, err: isErr })
    haptic(isErr ? 'error' : 'success')
    clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToastMsg(null), isErr ? 3600 : 2200)
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
  useEffect(() => () => clearTimeout(toastTimer.current), [])

  async function getOptions(a: number, region: string) {
    const key = a + '|' + region
    let o = optsCache.current[key]
    if (!o) {
      o = await api<Options>('/api/arv/options?acc=' + a + (region ? '&region=' + encodeURIComponent(region) : ''))
      optsCache.current[key] = o
    }
    return o
  }

  function vkey(v: View) {
    return v.v + ':' + ('domain' in v ? v.domain : '') + ('id' in v ? v.id : '')
  }
  function push(v: View) {
    scrollPos.current[vkey(view)] = window.scrollY
    setStack((s) => [...s, view])
    setView(v)
    setQ('')
    setRecQ('')
    requestAnimationFrame(() => window.scrollTo(0, 0))
  }
  function goBack() {
    if (modal) {
      setModal(null)
      return
    }
    if (view.v === 'create' && view.step > 1) {
      const step = view.step - 1
      setView({
        ...view,
        step,
        ...(step === 1
          ? { region: '', size: '', sizeName: '', image: '', imageName: '', firewall: '' }
          : step === 2
            ? { size: '', sizeName: '', image: '', imageName: '', firewall: '' }
            : { image: '', imageName: '', firewall: '' }),
      })
      return
    }
    if (stack.length) {
      const target = stack[stack.length - 1]
      scrollPos.current[vkey(view)] = window.scrollY
      setView(target)
      setStack(stack.slice(0, -1))
      requestAnimationFrame(() => window.scrollTo(0, scrollPos.current[vkey(target)] || 0))
      return
    }
    if (acc !== null) {
      setAcc(null)
      setTab('dns')
      setView({ v: 'accounts' })
      setStack([])
      setQ('')
      return
    }
    onSwitch('hz')
  }
  useEffect(() => {
    if (!TG || !TG.BackButton) return
    const h = () => goBack()
    TG.BackButton.onClick(h)
    return () => TG.BackButton.offClick && TG.BackButton.offClick(h)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, stack, modal, acc])
  useEffect(() => {
    if (TG && TG.BackButton) TG.BackButton.show()
  }, [view, stack, modal, acc])

  function reload() {
    setView((v) => ({ ...v }))
  }

  useEffect(() => {
    let cancelled = false
    setErr('')
    bump(1)
    ;(async () => {
      try {
        if ((view.v === 'dns' || view.v === 'servers') && acc === null) {
          const d = await api<{ accounts: Account[] }>('/api/arv/accounts')
          if (!cancelled) {
            setAccounts(d.accounts || [])
            if (d.accounts && d.accounts.length) setAcc(d.accounts[0].i)
          }
          return
        }
        if (view.v === 'accounts') {
          const d = await api<{ accounts: Account[] }>('/api/arv/accounts')
          if (!cancelled) setAccounts(d.accounts || [])
        } else if (view.v === 'dns' && acc !== null) {
          const d = await api<{ domains: Domain[] }>('/api/arv/domains?acc=' + acc)
          if (!cancelled) setDomains(d.domains || [])
        } else if (view.v === 'records' && acc !== null) {
          const d = await api<{ records: Record_[] }>(
            '/api/arv/records?acc=' + acc + '&domain=' + encodeURIComponent(view.domain),
          )
          if (!cancelled) setRecords(d.records || [])
        } else if (view.v === 'servers' && acc !== null) {
          const d = await api<{ servers: Server[] }>('/api/arv/servers?acc=' + acc)
          if (!cancelled) setServers(d.servers || [])
        } else if (view.v === 'server' && acc !== null) {
          const d = await api<{ servers: Server[] }>('/api/arv/servers?acc=' + acc)
          if (!cancelled) setServers(d.servers || [])
          const s = await api<{ snapshots: Snapshot[] }>(
            '/api/arv/snapshots?acc=' + acc + '&region=' + encodeURIComponent(view.region),
          )
          if (!cancelled) setSnapshots(s.snapshots || [])
        } else if (view.v === 'create' && acc !== null && view.step <= 3) {
          if (!cancelled) setOpts(null)
          const region = view.step >= 2 ? view.region : ''
          const o = await getOptions(acc, region)
          if (!cancelled) setOpts(o)
        }
      } catch (e: any) {
        if (!cancelled) {
          setErr(e.message || 'خطا')
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
  }, [view, acc])

  function openAccount(i: number) {
    setAcc(i)
    setTab('dns')
    setView({ v: 'dns' })
    setStack([])
    setQ('')
    requestAnimationFrame(() => window.scrollTo(0, 0))
  }

  function openTab(t: Tab) {
    setTab(t)
    setStack([])
    setQ('')
    setRecQ('')
    setRegionFilter('all')
    setView(t === 'dns' ? { v: 'dns' } : { v: 'servers' })
    requestAnimationFrame(() => window.scrollTo(0, 0))
  }

  function addRecord(rec?: Record_) {
    if (view.v !== 'records' || acc === null) return
    setModal({
      kind: 'form',
      title: rec ? 'ویرایش رکورد' : 'رکورد جدید',
      desc: rec ? `${rec.type} · ${rec.name}` : 'نوع، نام و مقدار رکورد را وارد کن.',
      fields: [
        {
          key: 'type',
          label: 'نوع',
          value: rec?.type || 'A',
          select: RECORD_TYPES.map((t) => ({ value: t, label: t })),
        },
        { key: 'name', label: 'نام', value: rec?.name || '', placeholder: '@ یا www' },
        { key: 'value', label: 'مقدار', value: rec?.value || '', placeholder: '1.2.3.4 یا host.example.com' },
        { key: 'ttl', label: 'TTL (اختیاری)', value: rec?.ttl ? String(rec.ttl) : '', placeholder: 'مثلاً 3600' },
      ],
      okText: rec ? 'ذخیره' : 'افزودن',
      onSubmit: async (v) => {
        const ttl = Number(v.ttl || 0)
        if (v.ttl && (!Number.isFinite(ttl) || ttl < 0)) {
          toast('TTL معتبر نیست', true)
          return
        }
        bump(1)
        try {
          const d = await api<{ message: string }>('/api/arv/record', {
            op: rec ? 'edit' : 'add',
            acc,
            domain: view.domain,
            id: rec?.id || '',
            type: v.type,
            name: v.name,
            value: v.value,
            ttl,
          })
          toast(d.message || 'ثبت شد')
          setRecQ('')
          setView((x) => ({ ...x }))
        } catch (e: any) {
          toast(e.message || 'خطا', true)
        } finally {
          bump(-1)
        }
      },
    })
  }

  async function toggleCloud(rec: Record_) {
    if (view.v !== 'records' || acc === null) return
    bump(1)
    try {
      const d = await api<{ message: string }>('/api/arv/record', {
        op: 'cloud',
        acc,
        domain: view.domain,
        id: rec.id,
        cloud: !rec.cloud,
      })
      toast(d.message || 'تغییر کرد')
      setView((x) => ({ ...x }))
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  function deleteRecord(rec: Record_) {
    if (view.v !== 'records' || acc === null) return
    setModal({
      kind: 'confirm',
      title: 'حذف رکورد',
      desc: `${rec.type} · ${rec.name} حذف شود؟`,
      danger: true,
      okText: 'حذف',
      onOk: async () => {
        bump(1)
        try {
          const d = await api<{ message: string }>('/api/arv/record', {
            op: 'delete', acc, domain: view.domain, id: rec.id,
          })
          toast(d.message || 'حذف شد')
          setView((x) => ({ ...x }))
        } catch (e: any) {
          toast(e.message || 'خطا', true)
        } finally {
          bump(-1)
        }
      },
    })
  }

  async function doAction(action: string, arg = '', confirm?: { title: string; desc?: string; danger?: boolean; okText?: string }) {
    if (view.v !== 'server' || acc === null) return
    const run = async () => {
      bump(1)
      try {
        const d = await api<{ message?: string; password?: string }>('/api/arv/action', {
          acc, region: view.region, id: view.id, action, arg, confirm: true,
        })
        if (d.password) {
          setModal({ kind: 'password', password: d.password, note: 'این رمز فقط یک‌بار نمایش داده می‌شود.' })
        } else {
          toast(d.message || 'انجام شد')
        }
        if (action === 'delete') {
          setStack([])
          setView({ v: 'servers' })
          return
        }
        setView((x) => ({ ...x }))
      } catch (e: any) {
        toast(e.message || 'خطا', true)
      } finally {
        bump(-1)
      }
    }
    if (confirm) {
      setModal({
        kind: 'confirm',
        title: confirm.title,
        desc: confirm.desc,
        danger: confirm.danger,
        okText: confirm.okText,
        onOk: run,
      })
    } else {
      await run()
    }
  }

  function snapshotFlow() {
    if (view.v !== 'server') return
    setModal({
      kind: 'prompt',
      title: 'نام اسنپ‌شات',
      placeholder: 'مثلاً before-upgrade',
      onOk: (v) => void doAction('snapshot', v, {
        title: 'ساخت اسنپ‌شات',
        desc: 'از سرور فعلی یک اسنپ‌شات ساخته شود؟',
      }),
    })
  }

  function submitCreate() {
    if (view.v !== 'create' || acc === null) return
    const name = createName.trim()
    if (!name) {
      toast('نام سرور را وارد کن', true)
      return
    }
    if (!/^[a-z0-9][a-z0-9-]{0,59}$/i.test(name)) {
      toast('نام فقط حروف انگلیسی، عدد و خط تیره باشد', true)
      return
    }
    const st = view
    setModal({
      kind: 'confirm',
      title: 'ساخت سرور آروان',
      desc: `«${name}» با پلن ${st.sizeName} در ${st.region} ساخته شود؟ ساخت سرور هزینه دارد.`,
      onOk: async () => {
        bump(1)
        try {
          const d = await api<{ message: string; password?: string }>('/api/arv/create', {
            acc,
            region: st.region,
            name,
            size: st.size,
            image: st.image,
            firewall: st.firewall || 'Default',
          })
          setServers([])
          if (d.password) {
            setModal({ kind: 'password', password: d.password, note: 'درخواست ساخت ارسال شد؛ چند دقیقه طول می‌کشد.' })
          } else {
            toast(d.message || 'درخواست ساخت ارسال شد')
          }
          setStack([])
          setTab('cloud')
          setView({ v: 'servers' })
          setCreateName('')
        } catch (e: any) {
          toast(e.message || 'خطا', true)
        } finally {
          bump(-1)
        }
      },
    })
  }

  const accInfo = acc !== null ? accounts.find((a) => a.i === acc) : null
  const currentServer =
    view.v === 'server' ? servers.find((s) => s.id === view.id && s.region === view.region) || null : null
  const title =
    view.v === 'accounts'
      ? 'آروان'
      : view.v === 'dns'
        ? 'دامنه‌ها'
        : view.v === 'records'
          ? view.domain
          : view.v === 'servers'
            ? 'سرورهای ابری'
            : view.v === 'server'
              ? 'جزئیات سرور'
              : 'ساخت سرور'

  function rootTabs() {
    const seg = (v: View, label: string) => (
      <button
        key={label}
        className={'cc-chip' + (view.v === v.v ? ' on' : '')}
        aria-pressed={view.v === v.v}
        onClick={() => goRoot(v)}
      >
        {label}
      </button>
    )
    return (
      <div className="cc-chips cc-mb" role="group" aria-label="بخش آروان">
        {seg({ v: 'servers' }, 'سرورهای ابری')}
        {seg({ v: 'dns' }, 'دامنه‌ها')}
      </div>
    )
  }

  function accChips() {
    if (accounts.length < 2) return null
    return (
      <div className="cc-chips cc-mb" role="group" aria-label="انتخاب اکانت آروان">
        {accounts.map((a) => (
          <button
            key={a.i}
            className={'cc-chip' + (acc === a.i ? ' on' : '')}
            aria-pressed={acc === a.i}
            onClick={() => {
              setAcc(a.i)
              setQ('')
              setRegionFilter('all')
            }}
          >
            {a.name}
          </button>
        ))}
      </div>
    )
  }

  function errorCard() {
    return (
      <div className="cc-card cc-error-card" role="alert">
        <div className="cc-error-ico"><WarningCircle size={24} /></div>
        <div className="cc-name">مشکلی پیش آمد</div>
        <div className="cc-sub">{err}</div>
        <Button variant="primary" icon={ArrowClockwise} onClick={reload}>تلاش دوباره</Button>
      </div>
    )
  }

  function accountsPage() {
    return (
      <>
        <div className="cc-pagebar">
          <Button variant="primary" icon={Plus} onClick={() => { setAccErr(''); setAccForm({ name: '', token: '' }) }}>
            افزودن اکانت آروان
          </Button>
        </div>
        {!accounts.length && busy > 0 && <Loader />}
        {!accounts.length && busy === 0 && (
          <Empty title="اکانتی ثبت نشده" description="با دکمهٔ بالا کلید API آروان را اضافه کن." />
        )}
        {accounts.map((a) => (
          <div key={a.i}>
            {([['servers', 'سرورهای ابری'], ['dns', 'دامنه‌ها']] as const).map(([v, label], k) => (
              <LayerCard
                key={v}
                className="cc-card cc-row cc-tap"
                role="button"
                tabIndex={0}
                onClick={() => {
                  setAcc(a.i)
                  setQ('')
                  goRoot({ v })
                }}
              >
                <span className="cc-avatar" style={k ? { opacity: 0.35 } : undefined}>
                  {(a.name || 'A').trim().charAt(0)}
                </span>
                <div className="cc-grow">
                  <div className="cc-name">{a.name || 'اکانت ' + (a.i + 1)}</div>
                  <div className="cc-sub">{label}</div>
                </div>
                <button
                  type="button"
                  className="cc-rowbtn danger"
                  aria-label="حذف اکانت"
                  onClick={(e) => {
                    e.stopPropagation()
                    setAccDel(a)
                  }}
                >
                  <Trash size={15} />
                </button>
                <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
              </LayerCard>
            ))}
          </div>
        ))}
        <div className="cc-hint cc-center cc-mt">آروان را به‌صورت یک Workspace واحد مدیریت کن.</div>
      </>
    )
  }

  function dnsPage() {
    const ql = q.trim().toLowerCase()
    const list = domains.filter((d) => !ql || d.name.toLowerCase().includes(ql))
    return (
      <>
        <Input className="cc-input cc-mb" placeholder="جستجوی دامنه" value={q} onChange={(e) => setQ(e.target.value)} />
        <div className="cc-grid2 cc-mb">
          <Button variant="outline" icon={ArrowClockwise} onClick={reload}>بروزرسانی</Button>
          <Button variant="outline" icon={ImageIcon} onClick={() => toast('حذف/افزودن دامنه از پنل آروان انجام می‌شود')}>
            راهنما
          </Button>
        </div>
        <div className="cc-section">{num(list.length)} دامنه</div>
        {domains.length === 0 && busy > 0 && <Loader />}
        {list.length === 0 && busy === 0 && <Empty title="دامنه‌ای پیدا نشد" />}
        {list.map((d) => (
          <LayerCard key={d.id || d.name} className="cc-card cc-row cc-tap" role="button" tabIndex={0} onClick={() => push({ v: 'records', domain: d.name })}>
            <span className="cc-avatar"><Globe size={17} /></span>
            <div className="cc-grow">
              <div className="cc-name cc-mono cc-copyline">
                <span>{d.name}</span>
                <CopyBtn value={d.name} onCopy={copy} label="کپی دامنه" />
              </div>
              <div className="cc-sub">{d.status ? `وضعیت: ${d.status}` : 'مدیریت رکوردهای DNS'}</div>
            </div>
            <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
          </LayerCard>
        ))}
      </>
    )
  }

  function recordsPage() {
    if (view.v !== 'records') return null
    const domain = view.domain
    const types = ['all', ...Array.from(new Set(records.map((r) => r.type)))]
    const ql = recQ.trim().toLowerCase()
    const list = records.filter(
      (r) => (recType === 'all' || r.type === recType) &&
        (!ql || (r.name + ' ' + r.value + ' ' + r.type).toLowerCase().includes(ql)),
    )
    return (
      <>
        <div className="cc-grid2 cc-mb">
          <Button variant="primary" icon={Plus} onClick={() => addRecord()}>رکورد</Button>
          <Button variant="outline" icon={ArrowClockwise} onClick={reload}>بروزرسانی</Button>
        </div>
        <Input className="cc-input cc-mb" placeholder="جستجوی رکوردها" value={recQ} onChange={(e) => setRecQ(e.target.value)} />
        {types.length > 2 && (
          <div className="cc-chips" role="group" aria-label="فیلتر نوع رکورد">
            {types.map((t) => (
              <button key={t} className={'cc-chip' + (recType === t ? ' on' : '')} aria-pressed={recType === t} onClick={() => setRecType(t)}>
                {t === 'all' ? 'همه' : t}
              </button>
            ))}
          </div>
        )}
        <div className="cc-section">{num(list.length)} رکورد</div>
        {records.length === 0 && busy > 0 && <Loader />}
        {list.length === 0 && busy === 0 && <Empty title="رکوردی نیست" />}
        {list.map((r) => (
          <LayerCard key={r.id} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name cc-mono cc-copyline">
                <span>
                  <span className={'cc-badge-inline cc-type cc-type-' + (r.type || '').toLowerCase()}>{r.type}</span> {r.name || '@'}
                </span>
                <CopyBtn value={recHost(r.name, domain)} onCopy={copy} label="کپی زیردامنه" />
              </div>
              <div className="cc-sub cc-mono cc-break cc-copyline">
                <span className="cc-copyval">{r.value}</span>
                <CopyBtn value={r.value} onCopy={copy} label="کپی مقدار" size={14} />
              </div>
            </div>
            <div className="cc-end">
              {r.cloud && <Badge variant="success" dot>ابری</Badge>}
              <div className="cc-row-actions">
                <Button
                  className={'cc-cloud-toggle' + (r.cloud ? ' on' : '')}
                  variant="ghost"
                  size="sm"
                  icon={Cloud}
                  aria-label={r.cloud ? 'حذف حالت ابری' : 'فعال‌کردن حالت ابری'}
                  aria-pressed={r.cloud}
                  onClick={() => void toggleCloud(r)}
                />
                <Button variant="ghost" size="sm" icon={PencilSimple} aria-label="ویرایش" onClick={() => addRecord(r)} />
                <Button variant="ghost" size="sm" icon={Trash} aria-label="حذف" onClick={() => deleteRecord(r)} />
              </div>
            </div>
          </LayerCard>
        ))}
      </>
    )
  }

  function serversPage() {
    const ql = q.trim().toLowerCase()
    const regions = Array.from(new Map(servers.map((s) => [s.region, s.region_name || s.region])).entries())
    const list = servers.filter(
      (s) =>
        (regionFilter === 'all' || s.region === regionFilter) &&
        (!ql || (s.name + ' ' + s.pub.join(' ') + ' ' + s.region_name + ' ' + s.region).toLowerCase().includes(ql)),
    )
    return (
      <>
        <div className="cc-grid2 cc-mb">
          <Button
            variant="primary"
            icon={Plus}
            onClick={() => {
              if (acc === null) return
              setCreateName('')
              push({ v: 'create', step: 1, region: '', size: '', sizeName: '', image: '', imageName: '', firewall: '' })
            }}
          >
            ساخت سرور
          </Button>
          <Button variant="outline" icon={ArrowClockwise} onClick={reload}>بروزرسانی</Button>
        </div>
        <Input className="cc-input cc-mb" placeholder="جستجوی نام یا آی‌پی" value={q} onChange={(e) => setQ(e.target.value)} />
        {regions.length > 1 && (
          <div className="cc-chips" role="group" aria-label="فیلتر منطقه">
            <button
              className={'cc-chip' + (regionFilter === 'all' ? ' on' : '')}
              aria-pressed={regionFilter === 'all'}
              onClick={() => setRegionFilter('all')}
            >
              همه
            </button>
            {regions.map(([id, label]) => (
              <button
                key={id}
                className={'cc-chip' + (regionFilter === id ? ' on' : '')}
                aria-pressed={regionFilter === id}
                onClick={() => setRegionFilter(id)}
              >
                {label}
              </button>
            ))}
          </div>
        )}
        <div className="cc-section">{num(list.length)} سرور</div>
        {servers.length === 0 && busy > 0 && <Loader />}
        {list.length === 0 && busy === 0 && <Empty title="سروری پیدا نشد" />}
        {list.map((s) => (
          <LayerCard
            key={s.region + s.id}
            className="cc-card cc-row cc-tap"
            role="button"
            tabIndex={0}
            onClick={() => push({ v: 'server', id: s.id, region: s.region })}
          >
            <div className="cc-grow">
              <div className="cc-name">{s.name}</div>
              <div className="cc-sub cc-ltr">
                {num(s.cpu)} vCPU · {num(s.ram)}GB RAM · {num(s.disk)}GB
                {s.region_name ? ' · ' + s.region_name : ''}
              </div>
              <div className="cc-sub cc-mono cc-mt4 cc-copyline">
                {s.pub[0] || s.priv[0] ? (
                  <>
                    <span className="cc-copyval">{s.pub[0] || s.priv[0]}</span>
                    <CopyBtn value={s.pub[0] || s.priv[0]} onCopy={copy} label="کپی آی‌پی" size={14} />
                  </>
                ) : (
                  <span>بدون آی‌پی</span>
                )}
              </div>
            </div>
            <div className="cc-end">
              <Badge variant={statusVariant(s.status)} dot>{s.status_fa || s.status}</Badge>
            </div>
          </LayerCard>
        ))}
      </>
    )
  }

  const sidx = currentServer ? servers.findIndex((x) => x.id === currentServer.id && x.region === currentServer.region) : -1
  async function pollHunt() {
    try {
      const d = await api<any>('/api/arv/hunt')
      setArvHunt(d)
    } catch {
      setArvHunt({ running: false })
    }
  }

  function serverPage() {
    if (view.v !== 'server') return null
    if (!currentServer) return busy > 0 ? <Loader /> : <Empty title="سرور پیدا نشد" />
    const s = currentServer
    const on = isOn(s.status)
    return (
      <>
        <LayerCard className="cc-card cc-row">
          <div className="cc-grow">
            <div className="cc-name">{s.name}</div>
            <div className="cc-sub">{s.image || s.os || '—'} · {s.region_name || s.region}</div>
          </div>
          <Badge variant={statusVariant(s.status)} dot>{s.status_fa || s.status}</Badge>
        </LayerCard>
        <LayerCard className="cc-card">
          <div className="cc-kv">
            <span>IPv4</span>
            <span className="cc-mono cc-copyline">
              <span className="cc-copyval">{(s.pub || []).find((x) => !x.includes(':')) || '—'}</span>
              {(s.pub || []).find((x) => !x.includes(':')) && (
                <CopyBtn
                  value={(s.pub || []).find((x) => !x.includes(':')) || ''}
                  onCopy={copy}
                  label="کپی IPv4"
                  size={14}
                />
              )}
            </span>
          </div>
          <div className="cc-kv">
            <span>IPv6</span>
            <span className="cc-mono cc-small cc-copyline">
              <span className="cc-copyval">{(s.pub || []).find((x) => x.includes(':')) || '—'}</span>
              {(s.pub || []).find((x) => x.includes(':')) && (
                <CopyBtn
                  value={(s.pub || []).find((x) => x.includes(':')) || ''}
                  onCopy={copy}
                  label="کپی IPv6"
                  size={14}
                />
              )}
            </span>
          </div>
          <div className="cc-kv"><span>پلن</span><span>{s.flavor || '—'}</span></div>
          <div className="cc-kv"><span>مشخصات</span><span className="cc-ltr">{num(s.cpu)} vCPU · {num(s.ram)}GB RAM · {num(s.disk)}GB</span></div>
          <div className="cc-kv"><span>سیستم</span><span>{s.os || '—'}</span></div>
          <div className="cc-kv"><span>ساخت</span><span>{faDate(s.created)}</span></div>
        </LayerCard>
        <div className="cc-actions">
          {on ? (
            <Button variant="secondary" icon={Power} onClick={() => void doAction('power-off')}>خاموش</Button>
          ) : (
            <Button variant="primary" icon={Power} onClick={() => void doAction('power-on')}>روشن</Button>
          )}
          <Button variant="secondary" icon={ArrowClockwise} onClick={() => void doAction('reboot')}>ری‌بوت</Button>
          <Button
            variant="secondary"
            icon={Key}
            onClick={() => void doAction('reset_root', '', {
              title: 'ریست رمز روت',
              desc: 'رمز جدید ساخته و فقط یک‌بار نمایش داده می‌شود.',
            })}
          >
            رمز روت
          </Button>
          <Button
            variant="secondary"
            icon={PencilSimple}
            onClick={() => setModal({
              kind: 'prompt',
              title: 'نام جدید سرور',
              value: s.name,
              onOk: (v) => void doAction('rename', v, { title: 'تغییر نام', desc: `نام به «${v}» تغییر کند؟` }),
            })}
          >
            تغییر نام
          </Button>
          <Button variant="secondary" icon={Camera} onClick={snapshotFlow}>اسنپ‌شات</Button>
          <Button
            variant="secondary"
            icon={Globe}
            disabled={arvIPBusy}
            onClick={async () => {
              setArvIPBusy(true)
              setArvIP({ ip: '', clean: true, verdict: 'در حال بررسی… (تا ۶۰ ثانیه)' })
              try {
                const d = await api<{ ip: string; clean: boolean; verdict: string }>(`/api/arv/ipcheck?i=${acc}&idx=${sidx}`)
                setArvIP(d)
              } catch (e: any) {
                setArvIP({ ip: '', clean: false, verdict: String(e?.message || e) })
              } finally {
                setArvIPBusy(false)
              }
            }}
          >
            بررسی IP
          </Button>
          <Button
            variant="secondary"
            icon={TrendUp}
            onClick={async () => {
              try {
                const d = await api<{ plans: any[] }>(`/api/arv/plans?i=${acc}&idx=${sidx}`)
                setArvPlans(d.plans || [])
              } catch (e: any) {
                toast(String(e?.message || e), true)
              }
            }}
          >
            ارتقای پلن
          </Button>
          <Button
            variant="secondary"
            icon={MagnifyingGlass}
            onClick={async () => {
              setArvHunt(null)
              setArvHuntOn(true)
              await pollHunt()
            }}
          >
            شکار پلن
          </Button>
          <Button
            className="cc-wide"
            variant="destructive"
            icon={Trash}
            onClick={() => void doAction('delete', '', {
              title: 'حذف سرور',
              desc: `سرور «${s.name}» برای همیشه حذف می‌شود.`,
              danger: true,
              okText: 'حذف',
            })}
          >
            حذف سرور
          </Button>
        </div>
        <div className="cc-section">اسنپ‌شات‌های منطقه ({num(snapshots.length)})</div>
        {snapshots.length === 0 && <div className="cc-hint cc-center">اسنپ‌شاتی در این منطقه نیست.</div>}
        {snapshots.map((sn) => (
          <LayerCard key={sn.id} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">{sn.name}</div>
              <div className="cc-sub">{sn.size ? faNum(sn.size) + ' GB · ' : ''}{faDate(sn.created)}</div>
            </div>
            <Button
              variant="secondary-destructive"
              size="sm"
              icon={Trash}
              aria-label="حذف اسنپ‌شات"
              onClick={() => void doAction('snapshot_delete', sn.id, {
                title: 'حذف اسنپ‌شات',
                desc: 'این اسنپ‌شات برای همیشه حذف می‌شود.',
                danger: true,
                okText: 'حذف',
              })}
            />
          </LayerCard>
        ))}
      </>
    )
  }

  function createPage() {
    if (view.v !== 'create') return null
    const c = view
    const steps = (
      <div className="cc-steps">
        {[1, 2, 3, 4].map((n) => <i key={n} className={c.step >= n ? 'on' : ''} />)}
      </div>
    )
    if (c.step < 4 && !opts) return <>{steps}<div className="cc-center cc-pad"><Loader /></div></>
    if (c.step === 1 && opts) {
      return (
        <>
          {steps}
          <div className="cc-section">منطقه</div>
          {opts.regions.map((r) => (
            <button key={r.id} className="cc-choice" onClick={() => setView({ ...c, step: 2, region: r.id })}>
              <span className="cc-grow">
                <span className="cc-name">{r.name}</span>
                <span className="cc-sub cc-mono">{r.id}</span>
              </span>
            </button>
          ))}
        </>
      )
    }
    if (c.step === 2 && opts) {
      const regionLabel = opts.regions.find((r) => r.id === c.region)?.name || c.region
      return (
        <>
          {steps}
          <div className="cc-section">پلن · {regionLabel}</div>
          {opts.sizes.length === 0 && <Empty title="پلنی برای این منطقه نیست" />}
          {opts.sizes.map((s) => (
            <button
              key={s.id}
              className="cc-choice"
              onClick={() => setView({ ...c, step: 3, size: s.id, sizeName: s.name })}
            >
              <span className="cc-grow">
                <span className="cc-name">{s.name}</span>
                <span className="cc-sub cc-ltr">{num(s.cpu)} vCPU · {num(s.ram)}GB RAM · {num(s.disk)}GB دیسک</span>
              </span>
              <span className="cc-sub">{s.price_text || (s.price ? num(s.price) + ' تومان' : '')}</span>
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
            <button key={im.id} className="cc-choice" onClick={() => setView({ ...c, step: 4, image: im.id, imageName: im.name })}>
              <span className="cc-grow">
                <span className="cc-name">{im.name}</span>
                <span className="cc-sub cc-mono">{im.id}</span>
              </span>
            </button>
          ))}
        </>
      )
    }
    return (
      <>
        {steps}
        <div className="cc-section">نام، فایروال و تأیید</div>
        <Input
          className="cc-input cc-mb"
          placeholder="نام سرور (مثلاً web-1)"
          value={createName}
          maxLength={60}
          onChange={(e) => setCreateName(e.target.value)}
        />
        {opts && opts.firewalls.length > 0 && (
          <>
            <div className="cc-section">گروه فایروال</div>
            <select
              className="cc-input cc-select cc-mb"
              value={c.firewall || opts.firewalls[0].name}
              onChange={(e) => setView({ ...c, firewall: e.target.value })}
            >
              {opts.firewalls.map((f) => <option key={f.id} value={f.name}>{f.name}</option>)}
            </select>
          </>
        )}
        <LayerCard className="cc-card">
          <div className="cc-kv">
            <span>منطقه</span>
            <span>{opts?.regions.find((r) => r.id === c.region)?.name || c.region}</span>
          </div>
          <div className="cc-kv"><span>پلن</span><span>{c.sizeName || c.size}</span></div>
          <div className="cc-kv"><span>تصویر</span><span className="cc-mono">{c.imageName || c.image}</span></div>
          <div className="cc-kv"><span>فایروال</span><span>{c.firewall || (opts?.firewalls[0]?.name || 'Default')}</span></div>
        </LayerCard>
        <Button className="cc-wide cc-submit" variant="primary" icon={Plus} onClick={submitCreate}>ساخت سرور</Button>
      </>
    )
  }

  let body: any
  if (err) body = errorCard()
  else if (view.v === 'accounts') body = accountsPage()
  else if (view.v === 'dns') body = <>{rootTabs()}{accChips()}{dnsPage()}</>
  else if (view.v === 'records') body = recordsPage()
  else if (view.v === 'servers') body = <>{rootTabs()}{accChips()}{serversPage()}</>
  else if (view.v === 'server') body = serverPage()
  else body = createPage()
  onBackRef?.(goBack)

  function goRoot(v: View) {
    setStack([])
    setView(v)
  }
  const arvSections: SubItem[] = [
    { id: 'accounts', label: 'اکانت‌ها', active: view.v === 'accounts', onSelect: () => goRoot({ v: 'accounts' }) },
    { id: 'dns', label: 'DNS', active: view.v === 'dns', onSelect: () => goRoot({ v: 'dns' }) },
    { id: 'servers', label: 'سرورها', active: view.v === 'servers', onSelect: () => goRoot({ v: 'servers' }) },
  ]

  return (
    <>
      <ProviderShell
        onTool={onTool}
        provider="arv"
        sections={arvSections}
        title={title}
        busy={busy > 0}
        showBack={!!modal || acc !== null || stack.length > 0}
        showSwitch={view.v === 'accounts'}
        onBack={goBack}
        onRefresh={reload}
        onChange={onSwitch}
        toast={toastMsg}
      >
        {body}
      </ProviderShell>
      <Dialog.Root open={!!accForm} onOpenChange={(o) => !o && setAccForm(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">افزودن اکانت آروان</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">
            کلید API را از پنل آروان (IAM → کلیدها) بساز و اینجا بگذار.
          </Dialog.Description>
          <div className="cc-form">
            <label className="cc-field">
              <span>نام اکانت</span>
              <Input
                value={accForm?.name || ''}
                onChange={(e) => setAccForm((f) => ({ name: e.target.value, token: f?.token || '' }))}
                placeholder="main"
                autoComplete="off"
              />
            </label>
            <label className="cc-field">
              <span>کلید API</span>
              <textarea
                className="cc-input cc-textarea"
                value={accForm?.token || ''}
                onChange={(e) => setAccForm((f) => ({ name: f?.name || '', token: e.target.value }))}
                placeholder="apikey xxxxxxxx-…"
                autoComplete="off"
              />
            </label>
            {accErr && <div className="cc-form-err">{accErr}</div>}
          </div>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setAccForm(null)}>انصراف</Button>
            <Button variant="primary" disabled={accBusy} onClick={submitAcc}>ذخیره</Button>
          </div>
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={!!accDel} onOpenChange={(o) => !o && setAccDel(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">حذف اکانت آروان</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">
            «{accDel?.name}» فقط از این پنل حذف می‌شود؛ دامنه‌ها و سرورهای آروان دست‌نخورده می‌مانند.
          </Dialog.Description>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setAccDel(null)}>انصراف</Button>
            <Button variant="primary" onClick={removeAcc}>حذف</Button>
          </div>
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={!!arvIP} onOpenChange={(o) => !o && setArvIP(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">بررسی IP</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">{arvIP?.ip || 'در حال استعلام'}</Dialog.Description>
          <div className={"cc-hint " + (arvIP?.clean ? '' : 'cc-err')} style={{ lineHeight: 1.9 }}>{arvIP?.verdict}</div>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setArvIP(null)}>بستن</Button>
          </div>
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={!!arvPlans} onOpenChange={(o) => !o && setArvPlans(null)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">ارتقای پلن</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">سرور باید خاموش باشد. تغییر پلن روی دیسک اثر ندارد.</Dialog.Description>
          {(arvPlans || []).map((p) => (
            <LayerCard key={p.id} className="cc-trow">
              <div className="cc-grow">
                <div className="cc-name">{p.name}</div>
                <div className="cc-sub cc-ltr">{num(p.cpu)} vCPU · {num(p.ram)}GB · {num(p.disk)}GB · {p.price}</div>
              </div>
              {p.current ? <Badge>فعلی</Badge> : (
                <Button
                  variant="outline"
                  onClick={async () => {
                    try {
                      await api('/api/arv/upgrade', { i: acc, idx: sidx, size: p.id })
                      toast('درخواست ارتقا ثبت شد')
                      setArvPlans(null)
                      reload()
                    } catch (e: any) {
                      toast(String(e?.message || e), true)
                    }
                  }}
                >انتخاب</Button>
              )}
            </LayerCard>
          ))}
          {arvPlans && arvPlans.length === 0 && <Empty title="پلنی پیدا نشد" />}
        </Dialog>
      </Dialog.Root>

      <Dialog.Root open={arvHuntOn} onOpenChange={(o) => !o && setArvHuntOn(false)}>
        <Dialog className="cc-dialog" size="base">
          <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
          <Dialog.Title className="cc-dialog-title">شکار پلن</Dialog.Title>
          <Dialog.Description className="cc-dialog-desc">هر دور چند سرور می‌سازد و اگر پلن موجود نبود دوباره تلاش می‌کند؛ نتیجه در تلگرام هم می‌آید.</Dialog.Description>
          <div className="cc-hint">{arvHunt?.running ? `در حال اجرا — دور ${num(arvHunt.round)}/${num(arvHunt.rounds)} · ساخته‌شده ${num(arvHunt.created)}` : 'غیرفعال'}</div>
          <div className="cc-field-row cc-mt">
            <label className="cc-field">
              <span>تعداد دور</span>
              <Input className="cc-input" type="number" defaultValue={6} id="arv-hunt-rounds" />
            </label>
            <label className="cc-field">
              <span>هر دور</span>
              <Input className="cc-input" type="number" defaultValue={1} id="arv-hunt-batch" />
            </label>
          </div>
          <div className="cc-dialog-actions">
            <Button variant="outline" onClick={() => setArvHuntOn(false)}>بستن</Button>
            <Button
              variant="primary"
              onClick={async () => {
                const r = Number((document.getElementById('arv-hunt-rounds') as HTMLInputElement)?.value || 6)
                const b = Number((document.getElementById('arv-hunt-batch') as HTMLInputElement)?.value || 1)
                try {
                  await api('/api/arv/hunt/start', { i: acc, region: currentServer?.region, idx: sidx, rounds: r, batch: b })
                  await pollHunt()
                } catch (e: any) {
                  toast(String(e?.message || e), true)
                }
              }}
            >شروع</Button>
            <Button
              variant="destructive"
              onClick={async () => {
                try {
                  await api('/api/arv/hunt/stop', {})
                  await pollHunt()
                } catch (e: any) {
                  toast(String(e?.message || e), true)
                }
              }}
            >توقف</Button>
          </div>
        </Dialog>
      </Dialog.Root>

      <ActionModalHost modal={modal} onClose={() => setModal(null)} onCopy={copy} />
    </>
  )
}

function ImageIcon({ size = 16, className }: { size?: number | string; className?: string }) {
  return <Cube size={size} className={className} />
}
