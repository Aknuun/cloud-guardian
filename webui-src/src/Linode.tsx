import { useEffect, useRef, useState } from 'react'
import {
  ArrowClockwise,
  ArrowRight,
  ArrowUp,
  Camera,
  CaretLeft,
  CheckCircle,
  Copy,
  GlobeHemisphereWest,
  Key,
  PencilSimple,
  Plus,
  Power,
  Scissors,
  Trash,
  WarningCircle,
  Wrench,
} from '@phosphor-icons/react'
import { Badge, Button, Empty, Input, LayerCard, Loader, Meter } from './components/ui'
import { CopyBtn } from './components/CopyBtn'
import { ProviderShell } from './components/ProviderShell'
import { ActionModalHost, type ActionModal } from './components/ActionModal'
import type { Provider, SubItem } from './components/Sidebar'
import type { ToolId } from './components/Tools'
import { api, fallbackCopy, faDate, faNum, haptic, mem, num, TG } from './lib'

type Account = { i: number; name: string; count?: number }
type Traffic = { ok: boolean; used_gb?: number; quota_gb?: number; billable_gb?: number; used_pct?: number }
type Snapshot = { id: string; name: string; description: string; size_gb: number; created: string }
type Server = {
  id: string
  name: string
  status: string
  raw_status: string
  ipv4: string
  ipv4_list: string[]
  ipv6: string
  region: string
  type: string
  image: string
  created: string
  group: string
  specs: { vcpus: number; ram_mb: number; disk_gb: number; transfer: number; gpus: number }
  traffic?: Traffic
  prices?: { monthly?: string; hourly?: string; label?: string }
  snapshots?: Snapshot[]
}
type Region = { id: string; label: string; country: string; site_type: string }
type Plan = { id: string; name: string; vcpus: number; ram_mb: number; disk_gb: number; transfer: number; class: string; monthly: string; hourly: string }
type Image = { id: string; name: string; description: string; vendor: string }
type Options = { regions: Region[]; types: Plan[]; images: Image[] }
type View =
  | { v: 'accounts' }
  | { v: 'servers'; acc: number }
  | { v: 'server'; acc: number; id: string }
  | { v: 'create'; acc: number; step: number; region: string; type: string; typeName: string; image: string; imageName: string }

const STATUS: Record<string, string> = {
  running: 'روشن',
  off: 'خاموش',
  starting: 'در حال روشن شدن',
  stopping: 'در حال خاموش شدن',
  rebuilding: 'در حال عملیات',
  deleting: 'در حال حذف',
  unknown: 'نامشخص',
}

function badgeVariant(st: string): 'success' | 'neutral' | 'warning' | 'error' | 'secondary' {
  if (st === 'running') return 'success'
  if (st === 'off') return 'neutral'
  if (st === 'starting' || st === 'stopping' || st === 'rebuilding') return 'warning'
  if (st === 'deleting' || st === 'unknown') return 'error'
  return 'secondary'
}

export default function Linode({
  onSwitch,
  onBackRef,
  onTool,
}: {
  onSwitch: (p: Provider) => void
  onBackRef?: (f: () => void) => void
  onTool?: (id: ToolId) => void
}) {
  const [view, setView] = useState<View>({ v: 'accounts' })
  const [stack, setStack] = useState<View[]>([])
  const [accounts, setAccounts] = useState<Account[]>([])
  const [servers, setServers] = useState<Server[]>([])
  const [server, setServer] = useState<Server | null>(null)
  const [opts, setOpts] = useState<Options | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(0)
  const [modal, setModal] = useState<ActionModal | null>(null)
  const [toastMsg, setToastMsg] = useState<{ t: string; err?: boolean } | null>(null)
  const [q, setQ] = useState('')
  const [createName, setCreateName] = useState('')
  const optsCache = useRef<Record<string, Options>>({})
  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const scrollPos = useRef<Record<string, number>>({})

  function bump(n: number) {
    setBusy((b) => Math.max(0, b + n))
  }
  function toast(t: string, isErr = false) {
    setToastMsg({ t, err: isErr })
    haptic(isErr ? 'error' : 'success')
    clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToastMsg(null), isErr ? 3400 : 2200)
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

  async function getOptions(acc: number, region: string) {
    const key = acc + '|' + region
    let o = optsCache.current[key]
    if (!o) {
      o = await api<Options>('/api/ln/options?acc=' + acc + (region ? '&region=' + encodeURIComponent(region) : ''))
      optsCache.current[key] = o
    }
    return o
  }

  function vkey(v: View) {
    return v.v + ':' + String((v as any).acc || '') + ':' + String((v as any).id || '')
  }
  function push(v: View) {
    scrollPos.current[vkey(view)] = window.scrollY
    setStack((s) => [...s, view])
    setView(v)
    setQ('')
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
          ? { region: '', type: '', typeName: '', image: '', imageName: '' }
          : step === 2
            ? { type: '', typeName: '', image: '', imageName: '' }
            : { image: '', imageName: '' }),
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
    onSwitch('hz')
  }
  useEffect(() => {
    if (!TG || !TG.BackButton) return
    const h = () => goBack()
    TG.BackButton.onClick(h)
    return () => TG.BackButton.offClick && TG.BackButton.offClick(h)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, stack, modal])
  useEffect(() => {
    if (TG && TG.BackButton) TG.BackButton.show()
  }, [view, stack, modal])
  useEffect(() => {
    try {
      sessionStorage.setItem('cg_ln_state', JSON.stringify({ view, stack }))
    } catch {
      /* ignore */
    }
  }, [view, stack])

  function reload() {
    setView((v) => ({ ...v }))
  }

  useEffect(() => {
    let cancelled = false
    setErr('')
    bump(1)
    ;(async () => {
      try {
        if (view.v === 'accounts') {
          const d = await api<{ accounts: Account[] }>('/api/ln/accounts')
          if (!cancelled) setAccounts(d.accounts || [])
        } else if (view.v === 'servers') {
          const d = await api<{ servers: Server[] }>('/api/ln/servers?acc=' + view.acc)
          if (!cancelled) setServers(d.servers || [])
        } else if (view.v === 'server') {
          const d = await api<{ server: Server }>(
            '/api/ln/server?acc=' + view.acc + '&id=' + encodeURIComponent(view.id),
          )
          if (!cancelled) setServer(d.server)
        } else if (view.v === 'create' && view.step <= 3) {
          if (!cancelled) setOpts(null)
          const region = view.step >= 2 ? view.region : ''
          const o = await getOptions(view.acc, region)
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
  }, [view])

  async function doAction(op: string, o?: { id?: string; arg?: string; confirm?: boolean; title?: string; desc?: string; danger?: boolean }) {
    if (view.v !== 'server' && view.v !== 'servers') return
    const acc = view.acc
    const id = o?.id || (view.v === 'server' ? view.id : '')
    const run = async () => {
      bump(1)
      try {
        const d = await api<{ password?: string; message?: string }>('/api/ln/action', {
          acc,
          id,
          action: op,
          arg: o?.arg || '',
          confirm: true,
        })
        if (d.password) {
          setModal({ kind: 'password', password: d.password, note: op === 'rebuild' ? 'برای ورود با این رمز، سرور را روشن نگه دار.' : undefined })
        } else {
          toast(d.message || 'انجام شد')
        }
        if (op === 'delete') {
          setStack([])
          setView({ v: 'servers', acc })
          return
        }
        if (view.v === 'server') {
          setServer(null)
          setView((v) => ({ ...v }))
        } else {
          setView((v) => ({ ...v }))
        }
      } catch (e: any) {
        toast(e.message || 'خطا', true)
      } finally {
        bump(-1)
      }
    }
    if (o?.confirm) {
      setModal({
        kind: 'confirm',
        title: o.title || 'تأیید عملیات',
        desc: o.desc,
        danger: o.danger ?? (op === 'delete' || op === 'rebuild' || op === 'snapshot_delete' || op.startsWith('ip_del')),
        onOk: run,
      })
    } else {
      await run()
    }
  }

  async function rebuildFlow() {
    if (view.v !== 'server' || !server) return
    bump(1)
    try {
      const o = await getOptions(view.acc, server.region)
      setModal({
        kind: 'picker',
        title: 'انتخاب تصویر برای ریبیلد',
        items: o.images.map((im) => ({ id: im.id, title: im.name, sub: im.vendor || im.description })),
        onPick: (img) =>
          setModal({
            kind: 'confirm',
            title: 'ریبیلد سرور',
            desc: 'دیسک سرور پاک و تصویر تازه نصب می‌شود. ادامه؟',
            danger: true,
            okText: 'ریبیلد',
            onOk: () => doAction('rebuild', { arg: img, confirm: false }),
          }),
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  async function resizeFlow() {
    if (view.v !== 'server' || !server) return
    bump(1)
    try {
      const o = await getOptions(view.acc, server.region)
      const cur = server.specs
      const plans = o.types.filter(
        (t) => t.ram_mb >= cur.ram_mb && t.vcpus >= cur.vcpus && t.disk_gb >= cur.disk_gb && t.id !== server.type,
      )
      setModal({
        kind: 'picker',
        title: 'ارتقای پلن',
        items: plans.map((t) => ({
          id: t.id,
          title: t.name,
          sub: `${num(t.vcpus)} vCPU · ${mem(t.ram_mb)} · ${num(t.disk_gb)}GB`,
          extra: t.monthly ? '$' + t.monthly + '/ماه' : '',
        })),
        onPick: (tid) =>
          setModal({
            kind: 'confirm',
            title: 'ارتقای پلن',
            desc: 'نوع سرور و دیسک ارتقا می‌یابد. ادامه؟',
            onOk: () => doAction('resize', { arg: tid, confirm: false }),
          }),
      })
    } catch (e: any) {
      toast(e.message || 'خطا', true)
    } finally {
      bump(-1)
    }
  }

  function submitCreate() {
    if (view.v !== 'create') return
    const name = createName.trim()
    if (!name) {
      toast('نام سرور را وارد کن', true)
      return
    }
    if (!/^[a-z0-9][a-z0-9-]{0,63}$/i.test(name)) {
      toast('نام فقط حروف انگلیسی، عدد و خط تیره باشد', true)
      return
    }
    const st = view
    setModal({
      kind: 'confirm',
      title: 'ساخت سرور',
      desc: `«${name}» با پلن ${st.typeName} در ${st.region} ساخته شود؟`,
      onOk: async () => {
        bump(1)
        try {
          const d = await api<{ password: string; ip: string; server: Server }>('/api/ln/create', {
            acc: st.acc,
            name,
            region: st.region,
            type: st.type,
            image: st.image,
          })
          setModal({
            kind: 'created',
            name,
            ip: d.ip,
            password: d.password,
            onDone: () => {
              setStack([])
              setView({ v: 'servers', acc: st.acc })
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

  const accName = (i: number) => accounts.find((a) => a.i === i)?.name || ''
  const title =
    view.v === 'accounts'
      ? 'لینود'
      : view.v === 'servers'
        ? 'سرورها' + (accName(view.acc) ? ' · ' + accName(view.acc) : '')
        : view.v === 'server'
          ? 'جزئیات سرور'
          : 'ساخت سرور'

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
        {!accounts.length && busy > 0 && <Loader />}
        {!accounts.length && busy === 0 && (
          <Empty title="اکانتی ثبت نشده" description="برای افزودن اکانت لینود از ربات استفاده کن." />
        )}
        {accounts.map((a) => (
          <LayerCard
            key={a.i}
            className="cc-card cc-row cc-tap"
            role="button"
            tabIndex={0}
            onClick={() => {
              setServers([])
              push({ v: 'servers', acc: a.i })
            }}
          >
            <span className="cc-avatar">{(a.name || 'L').trim().charAt(0)}</span>
            <div className="cc-grow">
              <div className="cc-name">{a.name || 'اکانت ' + (a.i + 1)}</div>
              <div className="cc-sub">{a.count === undefined ? 'برای دیدن سرورها بزن' : num(a.count) + ' سرور'}</div>
            </div>
            <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
          </LayerCard>
        ))}
        <div className="cc-hint cc-center cc-mt">هر اکانت را انتخاب کن تا سرورها و مشخصاتشان را ببینی.</div>
      </>
    )
  }

  function serversPage() {
    if (view.v !== 'servers') return null
    const ql = q.trim().toLowerCase()
    const list = servers.filter((s) => !ql || (s.name + ' ' + s.ipv4 + ' ' + s.region + ' ' + s.type).toLowerCase().includes(ql))
    return (
      <>
        <Input className="cc-input cc-mb" placeholder="جست‌وجوی نام، آی‌پی یا منطقه…" value={q} onChange={(e) => setQ(e.target.value)} />
        <div className="cc-mb">
          <Button
            className="cc-wide"
            variant="primary"
            icon={Plus}
            onClick={() => {
              setCreateName('')
              push({ v: 'create', acc: view.acc, step: 1, region: '', type: '', typeName: '', image: '', imageName: '' })
            }}
          >
            ساخت سرور
          </Button>
        </div>
        <div className="cc-section">{num(list.length)} سرور</div>
        {servers.length === 0 && busy > 0 && <Loader />}
        {list.length === 0 && busy === 0 && <Empty title="سروری پیدا نشد" />}
        {list.map((s) => (
          <LayerCard
            key={s.id}
            className="cc-card cc-row cc-tap"
            role="button"
            tabIndex={0}
            onClick={() => push({ v: 'server', acc: view.acc, id: s.id })}
          >
            <div className="cc-grow">
              <div className="cc-name">{s.name}</div>
              <div className="cc-sub cc-ltr">
                {s.type} · {num(s.specs.vcpus)} vCPU / {mem(s.specs.ram_mb)} / {num(s.specs.disk_gb)}GB
              </div>
              <div className="cc-sub cc-mono cc-mt4 cc-copyline">
                {s.ipv4 ? (
                  <>
                    <span className="cc-copyval">{s.ipv4}</span>
                    <CopyBtn value={s.ipv4} onCopy={copy} label="کپی IPv4" size={14} />
                  </>
                ) : s.ipv6 ? (
                  <>
                    <span className="cc-copyval">{s.ipv6}</span>
                    <CopyBtn value={s.ipv6} onCopy={copy} label="کپی IPv6" size={14} />
                  </>
                ) : (
                  <span>بدون آی‌پی</span>
                )}
                <span>· {s.region}</span>
              </div>
            </div>
            <div className="cc-end">
              <Badge variant={badgeVariant(s.status)} dot>{STATUS[s.status] || s.status}</Badge>
            </div>
          </LayerCard>
        ))}
      </>
    )
  }

  function serverPage() {
    if (view.v !== 'server') return null
    if (!server) return busy > 0 ? <Loader /> : null
    const s = server
    const tr = s.traffic
    const snaps = s.snapshots || []
    const on = s.status === 'running'
    return (
      <>
        <LayerCard className="cc-card cc-row">
          <div className="cc-grow">
            <div className="cc-name">{s.name}</div>
            <div className="cc-sub">{s.image || '—'} · {s.region} · {s.type}</div>
          </div>
          <Badge variant={badgeVariant(s.status)} dot>{STATUS[s.status] || s.status}</Badge>
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
          <div className="cc-kv"><span>پلن</span><span className="cc-ltr">{s.prices?.label || s.type} · {num(s.specs.vcpus)} vCPU / {mem(s.specs.ram_mb)} / {num(s.specs.disk_gb)}GB</span></div>
          <div className="cc-kv"><span>منطقه</span><span>{s.region}</span></div>
          <div className="cc-kv"><span>ساخت</span><span>{faDate(s.created)}</span></div>
          <div className="cc-kv"><span>هزینه</span><span>{s.prices?.monthly ? '$' + s.prices.monthly + ' ماهانه' : '—'}</span></div>
        </LayerCard>

        {tr?.ok && (
          <LayerCard className="cc-card">
            <Meter label="ترافیک مصرفی" value={Math.min(100, Number(tr.used_pct || 0))} max={100} showValue={false} />
            <div className="cc-sub cc-mt4">
              {faNum(tr.used_gb)} از {faNum(tr.quota_gb)} GB · {faNum(tr.used_pct)}٪ مصرف
              {Number(tr.billable_gb || 0) > 0 ? ` · ${faNum(tr.billable_gb)} GB مازاد` : ''}
            </div>
          </LayerCard>
        )}

        <div className="cc-actions">
          {on ? (
            <Button variant="secondary" icon={Power} onClick={() => doAction('shutdown')}>خاموش</Button>
          ) : (
            <Button variant="primary" icon={Power} onClick={() => doAction('boot')}>روشن</Button>
          )}
          <Button variant="secondary" icon={ArrowClockwise} onClick={() => doAction('reboot')}>ری‌بوت</Button>
          <Button
            variant="secondary"
            icon={Key}
            onClick={() =>
              setModal({
                kind: 'confirm',
                title: 'ریست رمز روت',
                desc: 'رمز جدید ساخته می‌شود؛ برای اعمال، سرور را خاموش و روشن کن.',
                onOk: () => doAction('reset_password', { confirm: false }),
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
                placeholder: 'مثلاً web-2',
                value: s.name,
                onOk: (v) => {
                  if (!v) return toast('نام خالی است', true)
                  void doAction('rename', { arg: v, confirm: false })
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
                title: 'نام اسنپ‌شات',
                placeholder: 'اختیاری',
                onOk: (v) => void doAction('snapshot_create', { arg: v, confirm: false }),
              })
            }
          >
            اسنپ‌شات
          </Button>
          <Button variant="outline" icon={Wrench} onClick={() => void rebuildFlow()}>ریبیلد</Button>
          <Button variant="outline" icon={ArrowUp} onClick={() => void resizeFlow()}>ارتقا</Button>
          <Button variant="outline" icon={GlobeHemisphereWest} onClick={() => doAction('ip_add4', { confirm: false })}>افزودن IPv4</Button>
          <Button variant="outline" icon={GlobeHemisphereWest} onClick={() => doAction('ip_add6', { confirm: false })}>افزودن IPv6</Button>
          <Button
            className="cc-wide"
            variant="secondary-destructive"
            icon={Scissors}
            onClick={() => doAction('ip_del4', { confirm: true, title: 'حذف IPv4 اضافه', desc: 'آخرین IPv4 عمومی اضافه حذف شود؟' })}
          >
            حذف IPv4 اضافه
          </Button>
          <Button
            className="cc-wide"
            variant="secondary-destructive"
            icon={Scissors}
            onClick={() => doAction('ip_del6', { confirm: true, title: 'حذف IPv6', desc: 'آخرین IPv6 این سرور حذف شود؟' })}
          >
            حذف IPv6
          </Button>
          <Button
            className="cc-wide"
            variant="destructive"
            icon={Trash}
            onClick={() => doAction('delete', { confirm: true, title: 'حذف سرور', desc: `سرور «${s.name}» برای همیشه حذف می‌شود.` })}
          >
            حذف سرور
          </Button>
        </div>

        <div className="cc-section">اسنپ‌شات‌ها ({num(snaps.length)})</div>
        {snaps.length === 0 && <div className="cc-hint cc-center">اسنپ‌شاتی نیست.</div>}
        {snaps.map((sn) => (
          <LayerCard key={sn.id} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">{sn.name || sn.id}</div>
              <div className="cc-sub">{faNum(sn.size_gb)} GB · {faDate(sn.created)}</div>
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
                  onOk: () => doAction('snapshot_delete', { arg: sn.id, confirm: false }),
                })
              }
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
                <span className="cc-name">{r.label}</span>
                <span className="cc-sub">{r.country || ''} · {r.id}</span>
              </span>
            </button>
          ))}
        </>
      )
    }
    if (c.step === 2 && opts) {
      const regionLabel = opts.regions.find((r) => r.id === c.region)?.label || c.region
      return (
        <>
          {steps}
          <div className="cc-section">پلن · {regionLabel}</div>
          {opts.types.length === 0 && <Empty title="پلنی برای این منطقه نیست" />}
          {opts.types.map((t) => (
            <button
              key={t.id}
              className="cc-choice"
              onClick={() => setView({ ...c, step: 3, type: t.id, typeName: t.name })}
            >
              <span className="cc-grow">
                <span className="cc-name">{t.name}</span>
                <span className="cc-sub cc-ltr">{num(t.vcpus)} vCPU · {mem(t.ram_mb)} RAM · {num(t.disk_gb)}GB · {num(t.transfer)}TB</span>
              </span>
              <span className="cc-sub">{t.monthly ? '$' + t.monthly : ''}</span>
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
                <span className="cc-sub">{im.vendor || ''} {im.description || ''}</span>
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
          placeholder="نام سرور (مثلاً web-1)"
          value={createName}
          maxLength={64}
          onChange={(e) => setCreateName(e.target.value)}
        />
        <LayerCard className="cc-card">
          <div className="cc-kv">
            <span>منطقه</span>
            <span>{opts?.regions.find((r) => r.id === c.region)?.label || c.region}</span>
          </div>
          <div className="cc-kv"><span>پلن</span><span>{c.typeName || c.type}</span></div>
          <div className="cc-kv"><span>تصویر</span><span className="cc-mono">{c.imageName || c.image}</span></div>
          <div className="cc-kv"><span>سیستم</span><span>تصویر انتخابی + رمز روت تصادفی</span></div>
        </LayerCard>
        <Button className="cc-wide cc-submit" variant="primary" icon={Plus} onClick={submitCreate}>ساخت سرور</Button>
      </>
    )
  }

  let body: any
  if (err) body = errorCard()
  else if (view.v === 'accounts') body = accountsPage()
  else if (view.v === 'servers') body = serversPage()
  else if (view.v === 'server') body = serverPage()
  else body = createPage()

  const showBack = !!modal || view.v !== 'accounts' || stack.length > 0

  function goRoot(v: View) {
    setStack([])
    setView(v)
  }
  const accId = 'acc' in view ? view.acc : null
  const sections: SubItem[] = [
    { id: 'accounts', label: 'اکانت‌ها', active: view.v === 'accounts', onSelect: () => goRoot({ v: 'accounts' }) },
  ]
  if (accId != null) {
    sections.push({
      id: 'servers',
      label: 'سرورها',
      active: view.v === 'servers' || view.v === 'server',
      onSelect: () => goRoot({ v: 'servers', acc: accId }),
    })
  }
  const showSwitch = view.v === 'accounts' || view.v === 'servers'
  onBackRef?.(goBack)

  return (
    <>
      <ProviderShell
        onTool={onTool}
        sections={sections}
        provider="ln"
        title={title}
        busy={busy > 0}
        showBack={showBack}
        showSwitch={showSwitch}
        onBack={goBack}
        onRefresh={reload}
        onChange={onSwitch}
        toast={toastMsg}
      >
        {body}
      </ProviderShell>
      <ActionModalHost modal={modal} onClose={() => setModal(null)} onCopy={copy} />
    </>
  )
}
