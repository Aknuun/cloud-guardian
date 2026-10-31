import { useEffect, useRef, useState } from 'react'
import { Badge, Button, Dialog, Empty, LayerCard, Switch } from './components/ui'
import type { Provider, SubItem } from './components/Sidebar'
import type { ToolId } from './components/toolsMeta'
import { ProviderShell } from './components/ProviderShell'
import {
  ArrowRight,
  ArrowsClockwise,
  Broom,
  CaretDown,
  CaretLeft,
  ChartBar,
  CheckSquare,
  Clock,
  EnvelopeSimple,
  Gear,
  Globe,
  Info,
  Lifebuoy,
  Link as LinkIcon,
  MagnifyingGlass,
  Pause,
  PencilSimple,
  Play,
  Plus,
  SlidersHorizontal,
  Star,
  Trash,
  UsersThree,
  X,
} from '@phosphor-icons/react'
import { api, checkRecord, fallbackCopy, faDate, faNum, haptic, num, TG } from './lib'
import { CopyBtn } from './components/CopyBtn'
import type { CFAcc, CFRec, CFZone, Dlg, EmailData, FavItem, LbEntry, LbGroup, SetGroup, SetItem, Tab, View, ZoneInfo } from './cloudflare/model'
import {
  PROXYABLE,
  TAB_KEYS,
  ZONE_BADGE,
  isEmail,
  isHost,
  isIPv4,
  isIPv6,
  relName,
  restoreCfState,
  riskySetting,
  splitFavorites,
  statusLabel,
  ttlLabel,
} from './cloudflare/model'
import {
  ConvertRecordDialog,
  ErrorState,
  FavStar,
  FormDialog,
  RecordEditor,
  RecordSheet,
  RefTreeDialog,
  SearchInput,
  SkeletonList,
  SpareSheet,
} from './cloudflare/components'

export default function Cloudflare({
  onSwitch,
  onBackRef,
  onTool,
}: {
  onSwitch: (p: Provider) => void
  onBackRef: (f: () => void) => void
  onTool?: (id: ToolId) => void
}) {
  const [boot] = useState(restoreCfState)
  const [view, setView] = useState<View>(boot?.view || { v: 'zones' })

  // Opened from the record health page: jump straight to that record's editor.
  useEffect(() => {
    try {
      const raw = sessionStorage.getItem('cf_edit')
      if (!raw) return
      sessionStorage.removeItem('cf_edit')
      const v = JSON.parse(raw)
      if (typeof v?.acc === 'number' && v?.zid && v?.rid) {
        setPendingEdit({ acc: v.acc, zid: v.zid, rid: v.rid })
        setView({ v: 'zone', acc: v.acc, zid: v.zid })
      }
    } catch {
      /* ignore */
    }
  }, [])
  const [stack, setStack] = useState<View[]>(boot?.stack || [])
  const [rev, setRev] = useState(0)
  const [busy, setBusy] = useState(0)
  const [toastMsg, setToastMsg] = useState<{ t: string; err?: boolean } | null>(null)
  const [dlg, setDlg] = useState<Dlg | null>(null)
  const [q, setQ] = useState('')
  const [recQ, setRecQ] = useState('')
  const [recType, setRecType] = useState('all')
  const [newId, setNewId] = useState('')
  const [searchErr, setSearchErr] = useState('')
  const [searchBusy, setSearchBusy] = useState(false)
  const [tab, setTab] = useState<Tab>(boot?.tab || 'records')
  const [err, setErr] = useState('')

  const [zones, setZones] = useState<CFZone[]>([])
  const [accs, setAccs] = useState<CFAcc[]>([])
  const [accFilter, setAccFilter] = useState<number>(-1)

  const [zone, setZone] = useState<ZoneInfo | null>(null)
  const [records, setRecords] = useState<CFRec[]>([])
  const bulkWarn = useRef('')
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
  const [pendingEdit, setPendingEdit] = useState<{ acc: number; zid: string; rid: string } | null>(null)
  const [pendingEditor, setPendingEditor] = useState<{ acc: number; zid: string; zone: string } | null>(null)
  const [days, setDays] = useState(1)

  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const scrollPos = useRef<Record<string, number>>({})
  const fromSearch = useRef(false)
  const favPending = useRef<Set<string>>(new Set())
  const bump = (n: number) => setBusy((b) => Math.max(0, b + n))
  function toast(t: string, isErr = false) {
    setToastMsg({ t, err: isErr })
    haptic(isErr ? 'error' : 'success')
    clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToastMsg(null), isErr ? 3200 : 2100)
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

  function vkey(v: View) {
    return v.v + ':' + String((v as any).acc || '') + ':' + String((v as any).zid || '') + ':' + String((v as any).name || '')
  }
  function saveScroll() {
    scrollPos.current[vkey(view)] = window.scrollY
  }
  function jumpTo(y: number) {
    requestAnimationFrame(() => window.scrollTo(0, Math.max(0, y)))
  }
  function push(v: View) {
    saveScroll()
    setStack((s) => [...s, view])
    setView(v)
    setErr('')
    if (v.v === 'search') setQ('')
    if (v.v === 'zone') {
      setTab('records')
      setRecQ('')
      setRecType('all')
      setSel(new Set())
      setSelMode(false)
      setSettings(null)
      setEmail(null)
      setTraffic(null)
    }
    jumpTo(0)
  }
  function goBack() {
    if (dlg) {
      setDlg(null)
      leaveSearchDialog()
      return
    }
    if (stack.length) {
      const target = stack[stack.length - 1]
      saveScroll()
      setView(target)
      setStack(stack.slice(0, -1))
      setErr('')
      jumpTo(scrollPos.current[vkey(target)] || 0)
      return
    }
    onSwitch('hz')
  }
  function openTab(k: Tab) {
    saveScroll()
    setTab(k)
    if (k === 'records') setRecQ('')
    setErr('')
    jumpTo(0)
  }
  onBackRef(goBack)
  useEffect(() => {
    if (TG && TG.BackButton) TG.BackButton.show()
  }, [view, stack.length, dlg])
  useEffect(() => {
    try {
      sessionStorage.setItem('cg_cf_state', JSON.stringify({ view, stack, tab }))
    } catch {
      /* ignore */
    }
  }, [view, stack, tab])

  /* ---------------- data ---------------- */

  useEffect(() => {
    let cancelled = false
    const ac = new AbortController()
    setErr('')
    bump(1)
    ;(async () => {
      try {
        if (view.v === 'zones' || view.v === 'lbzones') {
          const d = await api<{ zones: CFZone[]; accounts: CFAcc[] }>('/api/cf/zones', undefined, ac.signal)
          if (!cancelled) {
            setZones(d.zones || [])
            setAccs(d.accounts || [])
          }
        } else if (view.v === 'accounts') {
          const d = await api<{ accounts: CFAcc[] }>('/api/cf/accounts', undefined, ac.signal)
          if (!cancelled) setAccs(d.accounts || [])
        } else if (view.v === 'favs') {
          const d = await api<{ items: FavItem[] }>('/api/cf/favs', undefined, ac.signal)
          if (!cancelled) setFavs(d.items || [])
        } else if (view.v === 'zone') {
          const d = await api<{ zone: ZoneInfo; records: CFRec[] }>(
            '/api/cf/records?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid),
            undefined,
            ac.signal,
          )
          if (cancelled) return
          setZone(d.zone)
          setRecords(d.records || [])
          if (pendingEdit && pendingEdit.acc === view.acc && pendingEdit.zid === view.zid) {
            const hit = (d.records || []).find((r) => r.id === pendingEdit.rid)
            if (hit) setDlg({ k: 'editor', acc: view.acc, zid: view.zid, zone: d.zone.name, rec: hit })
            setPendingEdit(null)
          }
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
            const s = await api<any>('/api/cf/settings?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid), undefined, ac.signal)
            if (!cancelled) setSettings({ groups: s.groups || [], dev: !!s.dev, paused: !!s.paused })
          } else if (tab === 'email') {
            const e = await api<EmailData>('/api/cf/email?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid), undefined, ac.signal)
            if (!cancelled) setEmail(e)
          } else if (tab === 'traffic') {
            const t = await api<any>(
              '/api/cf/traffic?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid) + '&days=' + days,
              undefined,
              ac.signal,
            )
            if (!cancelled) setTraffic({ days: t.days, total: t.total, top: t.top || [] })
          }
        } else if (view.v === 'lb') {
          const d = await api<{ zone: ZoneInfo; groups: LbGroup[] }>(
            '/api/cf/lbgroups?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid),
            undefined,
            ac.signal,
          )
          if (!cancelled) {
            setZone(d.zone)
            setLbGroups(d.groups || [])
          }
        } else if (view.v === 'lbgroup') {
          const d = await api<{ zone: ZoneInfo; name: string; entries: LbEntry[] }>(
            '/api/cf/lb?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid) + '&name=' + encodeURIComponent(view.name),
            undefined,
            ac.signal,
          )
          if (!cancelled) {
            setZone(d.zone)
            setLbName(d.name)
            setLbEntries(d.entries || [])
          }
        }
      } catch (e: any) {
        if (!cancelled && e && e.name !== 'AbortError') {
          setErr(e.message || 'خطا')
          toast(e.message || 'خطا', true)
        }
      } finally {
        bump(-1)
      }
    })()
    return () => {
      cancelled = true
      ac.abort()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, rev, tab, days, pendingRid])

  function refresh() {
    setRev((r) => r + 1)
  }
  useEffect(() => {
    if (!newId) return
    const el = document.getElementById('rec-' + newId)
    if (el) requestAnimationFrame(() => el.scrollIntoView({ block: 'center', behavior: 'smooth' }))
    const t = setTimeout(() => setNewId(''), 3200)
    return () => clearTimeout(t)
  }, [newId, records])
  useEffect(() => {
    if (view.v !== 'search' && view.v !== 'zones') return
    const term = q.trim()
    setSearchErr('')
    if (term.length < 2) {
      setSearch(null)
      setSearchBusy(false)
      return
    }
    let cancelled = false
    const ac = new AbortController()
    setSearchBusy(true)
    const t = setTimeout(async () => {
      try {
        const d = await api<{ zones: CFZone[]; records: any[] }>(
          '/api/cf/search?q=' + encodeURIComponent(term),
          undefined,
          ac.signal,
        )
        if (!cancelled) setSearch({ zones: d.zones || [], records: d.records || [] })
      } catch (e: any) {
        if (!cancelled && e && e.name !== 'AbortError') setSearchErr(e.message || 'خطا')
      } finally {
        if (!cancelled) setSearchBusy(false)
      }
    }, 320)
    return () => {
      cancelled = true
      clearTimeout(t)
      ac.abort()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, q, rev])


  async function post(path: string, body: any, keep = false, opts: { refresh?: boolean } = {}) {
    bump(1)
    try {
      const d = await api<any>(path, body)
      toast(d.message || 'انجام شد')
      if (opts.refresh !== false) setRev((r) => r + 1)
      if (!keep) setDlg(null)
      return d
    } catch (e: any) {
      toast(e.message || 'خطا', true)
      return null
    } finally {
      bump(-1)
    }
  }

  function patchRec(rec: CFRec) {
    setRecords((list) => list.map((r) => (r.id === rec.id ? { ...r, ...rec } : r)))
    setSearch((s) => (s ? { ...s, records: s.records.map((r) => (r.id === rec.id ? { ...r, ...rec } : r)) } : s))
    setDlg((d) => (d && d.k === 'record' && d.rec.id === rec.id ? { ...d, rec: { ...d.rec, ...rec } } : d))
  }

  function addRec(rec: CFRec) {
    setRecords((list) => [...list.filter((r) => r.id !== rec.id), rec])
  }

  function dropRec(rid: string) {
    setRecords((list) => list.filter((r) => r.id !== rid))
    setSearch((s) => (s ? { ...s, records: s.records.filter((r) => r.id !== rid) } : s))
    setFavs((list) => list.filter((f) => f.rid !== rid))
    setSel((s) => {
      if (!s.has(rid)) return s
      const n = new Set(s)
      n.delete(rid)
      return n
    })
  }

  function fqdn(name: string, zname: string) {
    const n = name.trim().toLowerCase()
    if (!n) return ''
    if (n === '@') return zname
    if (n === zname || n.endsWith('.' + zname)) return n
    return n + '.' + zname
  }

  function patchLb(content: string, patch: Partial<LbEntry>) {
    setLbEntries((list) => list.map((e) => (e.content === content ? { ...e, ...patch } : e)))
  }

  function addLb(entry: LbEntry) {
    setLbEntries((list) => [...list, entry])
  }

  function dropLb(content: string) {
    setLbEntries((list) => list.filter((e) => e.content !== content))
  }

  /* ---------------- record actions ---------------- */

  function recBody(rec: CFRec, acc: number, zid: string) {
    return { acc, zid, rid: rec.id }
  }

  function applyFav(rid: string, fav: boolean) {
    setRecords((list) => list.map((r) => (r.id === rid ? { ...r, fav } : r)))
    setSearch((s) => (s ? { ...s, records: s.records.map((r) => (r.id === rid ? { ...r, fav } : r)) } : s))
    setDlg((d) => (d && d.k === 'record' && d.rec.id === rid ? { ...d, rec: { ...d.rec, fav } } : d))
    if (!fav) setFavs((list) => list.filter((f) => f.rid !== rid))
  }

  async function setFav(acc: number, zid: string, rid: string, next: boolean) {
    if (favPending.current.has(rid)) return
    favPending.current.add(rid)
    applyFav(rid, next)
    bump(1)
    try {
      const d = await api<any>('/api/cf/record', { op: 'fav', acc, zid, rid })
      toast(d.message || (next ? 'به منتخب اضافه شد' : 'از منتخب حذف شد'))
    } catch (e: any) {
      applyFav(rid, !next)
      toast(e.message || 'خطا', true)
    } finally {
      favPending.current.delete(rid)
      bump(-1)
    }
  }

  function toggleFav(acc: number, zid: string, rec: CFRec) {
    void setFav(acc, zid, rec.id, !rec.fav)
  }

  async function toggleProxy(acc: number, zid: string, rec: CFRec) {
    const next = !rec.proxied
    patchRec({ ...rec, proxied: next })
    const d = await post('/api/cf/record', { ...recBody(rec, acc, zid), op: 'proxy', proxied: next }, true, { refresh: false })
    if (d && d.record) patchRec(d.record)
    else if (!d) patchRec(rec)
  }

  async function cycleTTL(acc: number, zid: string, rec: CFRec) {
    const steps = [1, 120, 300, 1800, 3600, 21600, 86400]
    const i = steps.indexOf(rec.ttl)
    const next = i < 0 ? 1 : steps[(i + 1) % steps.length]
    patchRec({ ...rec, ttl: next })
    const d = await post('/api/cf/record', { ...recBody(rec, acc, zid), op: 'ttl', ttl: 0 }, true, { refresh: false })
    if (d && d.record) patchRec(d.record)
    else if (!d) patchRec(rec)
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
        title: 'ترافیک امروز',
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
        k: 'tree',
        title: 'CNAMEهای وارده',
        target: d.target || rec.name,
        zone: d.zone || '',
        nodes: d.tree || [],
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
  function confirmBulkDelete() {
    if (sel.size === 0) return
    setDlg({
      k: 'confirm',
      title: 'حذف گروهی رکوردها',
      desc: `${faNum(sel.size)} رکورد انتخاب‌شده برای همیشه حذف می‌شوند.`,
      danger: true,
      okText: 'حذف',
      onOk: () => bulk('del'),
    })
  }

  function bulkEdit() {
    setDlg({
      k: 'form',
      title: 'تغییر مقدار',
      desc: 'مقدار جدید برای همهٔ رکوردهای انتخاب‌شده اعمال می‌شود.',
      fields: [{ key: 'content', label: 'مقدار جدید', placeholder: '1.2.3.4 یا host.example.com' }],
      okText: 'اعمال',
      onSubmit: async (v) => {
        const types = [...new Set(records.filter((r) => sel.has(r.id)).map((r) => r.type))]
        const key = v.content.trim()
        if (bulkWarn.current !== key) {
          for (const t of types) {
            const chk = await checkRecord(t, key)
            if (!chk.ok) {
              bulkWarn.current = key
              haptic('error')
              toast(chk.message + ' — اگر مطمئنی، دوباره بزن تا اعمال شود.', true)
              return
            }
          }
        }
        await bulk('edit', { content: v.content })
      },
    })
  }
  function bulkType() {
    setDlg({
      k: 'pick',
      title: 'تغییر نوع رکوردها',
      items: ['A', 'AAAA', 'CNAME'].map((t) => ({ id: t, title: t })),
      onPick: (t) => {
        setDlg({
          k: 'confirm',
          title: 'تغییر نوع گروهی',
          desc: `نوع ${faNum(sel.size)} رکورد به ${t} تغییر می‌کند؛ مقدارها باید با نوع جدید سازگار باشند.`,
          danger: true,
          okText: 'تغییر نوع',
          onOk: () => bulk('type', { type: t }),
        })
      },
    })
  }

  /* ---------------- settings/email helpers ---------------- */

  function setSetting(key: string, value: any) {
    if (view.v !== 'zone') return
    void post('/api/cf/settings', { op: 'set', acc: view.acc, zid: view.zid, key, value })
  }
  function applySetting(it: SetItem, value: string) {
    const reason = riskySetting(it.key, value, it.value)
    if (!reason) {
      setSetting(it.key, value)
      return
    }
    const label = it.values?.find((v) => v.v === value)?.l || (value === 'on' ? 'روشن' : value === 'off' ? 'خاموش' : value)
    setDlg({
      k: 'confirm',
      title: 'تغییر تنظیم حساس',
      desc: `${it.label} به «${label}» تغییر کند؟
${reason}`,
      danger: true,
      okText: 'اعمال',
      onOk: () => setSetting(it.key, value),
    })
  }

  function openZoneDetails() {
    if (!zone) return
    setDlg({
      k: 'list',
      title: 'جزئیات دامنه',
      rows: [
        { title: zone.name, sub: 'نام' },
        { title: zone.id, sub: 'شناسه' },
        { title: statusLabel(zone.status), sub: 'وضعیت' },
        { title: zone.plan || '—', sub: 'پلن' },
        { title: faDate(zone.created), sub: 'ساخت' },
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
      title: 'مقصد catch-all',
      items: [
        ...verified.map((d) => ({ id: d.email, title: d.email })),
        { id: '__off', title: 'غیرفعال کردن', sub: 'catch-all خاموش شود' },
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
        <SearchInput placeholder="جست‌وجوی دامنه، زیردامنه یا آی‌پی…" value={q} onChange={setQ} />
        <div className="cc-chips" role="group" aria-label="فیلتر اکانت">
          <button
            className={'cc-chip' + (accFilter === -1 ? ' on' : '')}
            aria-pressed={accFilter === -1}
            onClick={() => setAccFilter(-1)}
          >
            همه اکانت‌ها
          </button>
          {accs.map((a) => (
            <button
              key={a.i}
              className={'cc-chip' + (accFilter === a.i ? ' on' : '')}
              aria-pressed={accFilter === a.i}
              onClick={() => setAccFilter(a.i)}
            >
              {a.name}
            </button>
          ))}
        </div>
        {(count > 0 || !(view.v === 'zones' && ql.length >= 2)) && <div className="cc-section">{num(count)} دامنه</div>}
        {count === 0 && !busy && !err && !(view.v === 'zones' && ql.length >= 2) && <Empty title={emptyText} />}
        {zones.length === 0 && busy > 0 && <SkeletonList />}
        {filtered.map((z) => (
          <LayerCard
            key={z.id}
            className="cc-card cc-row cc-tap"
            role="button"
            tabIndex={0}
            onClick={() => (onPick ? onPick(z) : push({ v: 'zone', acc: z.acc, zid: z.id }))}
          >
            <div className="cc-grow">
              <div className="cc-name cc-mono">{z.name}</div>
              <div className="cc-sub">{z.accName || 'اکانت ' + (z.acc + 1)}</div>
            </div>
            <Badge variant={ZONE_BADGE[z.status] || 'secondary'} dot>
              {statusLabel(z.status)}
            </Badge>
            <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
          </LayerCard>
        ))}
        {view.v === 'zones' && ql.length >= 2 && recordHitsUI()}
      </>
    )
  }

  function recordHitsUI() {
    const hits = (search?.records || []).filter((r) => accFilter === -1 || r.acc === accFilter)
    return (
      <>
        <div className="cc-section">
          {searchBusy && !search ? 'جست‌وجوی رکوردها…' : num(hits.length) + ' رکورد'}
        </div>
        {searchBusy && !search && <SkeletonList rows={2} />}
        {searchErr && <div className="cc-hint cc-center">جست‌وجوی رکوردها ممکن نشد.</div>}
        {!searchBusy && search && hits.length === 0 && (
          <div className="cc-hint cc-center">رکوردی با این نام یا آی‌پی پیدا نشد.</div>
        )}
        {hits.map((r) => (
          <LayerCard
            key={r.zid + ':' + r.id}
            className="cc-card cc-row cc-tap"
            onClick={() => openInZone(r.acc, r.zid, r.zone, r.id)}
          >
            <Badge variant="secondary" className={'cc-type cc-type-' + r.type.toLowerCase()}>{r.type}</Badge>
            <div className="cc-grow">
              <div className="cc-name cc-mono cc-copyline">
                <span>{relName(r.name, r.zone)}</span>
                <CopyBtn value={r.name} onCopy={copy} label="کپی زیردامنه" />
              </div>
              <div className="cc-sub cc-mono cc-copyline">
                <span className="cc-copyval">{r.content}</span>
                <CopyBtn value={r.content} onCopy={copy} label="کپی مقدار" size={14} />
              </div>
              {r.match === 'cname' && <div className="cc-sub">CNAME مرتبط با این IP</div>}
            </div>
            <div className="cc-sub">{r.zone}</div>
          </LayerCard>
        ))}
      </>
    )
  }

  function zonesPage() {
    return (
      <>
        <Button
          className="cc-primary-action"
          variant="primary"
          size="lg"
          icon={Plus}
          onClick={() => {
            const active = zones.filter((z) => z.status === 'active')
            setDlg({
              k: 'pick',
              title: 'افزودن رکورد به کدام دامنه؟',
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
          افزودن رکورد
        </Button>
        <div className="cc-action-grid cc-mb">
          <Button variant="outline" icon={Star} onClick={() => push({ v: 'favs' })}>
            منتخب‌ها
          </Button>
          <Button variant="outline" icon={MagnifyingGlass} onClick={() => push({ v: 'search' })}>
            جست‌وجوی رکورد
          </Button>
          <Button variant="outline" icon={Lifebuoy} onClick={() => push({ v: 'lbzones' })}>
            لود بالانسر
          </Button>
          <Button variant="outline" icon={UsersThree} onClick={() => push({ v: 'accounts' })}>
            اکانت‌ها
          </Button>
        </div>
        {zonesBody(zones)}
      </>
    )
  }

  /* ---------------- favorites/search ---------------- */

  // Opening a result from the global search must come back to the search, not the zone.
  function leaveSearchDialog() {
    if (!fromSearch.current) return
    fromSearch.current = false
    if (stack.length) {
      saveScroll()
      setView(stack[stack.length - 1])
      setStack(stack.slice(0, -1))
      setErr('')
    }
  }

  function openInZone(acc: number, zid: string, zoneName: string, rid: string) {
    if (view.v === 'search') fromSearch.current = true
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
        <div className="cc-section">رکوردهای ستاره‌دار</div>
        {favs.length === 0 && !busy && !err && (
          <Empty title="منتخب‌ها خالی است" description="از فهرست رکوردها یا داخل رکورد، ستاره را بزن." />
        )}
        {favs.length === 0 && busy > 0 && <SkeletonList />}
        {favs.map((f) => (
          <LayerCard
            key={f.rid}
            className={'cc-card cc-row' + (f.missing ? '' : ' cc-tap')}
            onClick={() => {
              if (!f.missing) openInZone(f.acc, f.zid, f.zone, f.rid)
            }}
          >
            <Badge variant="secondary">{f.type || '—'}</Badge>
            <div className="cc-grow">
              <div className="cc-name cc-copyline">
                <span>{f.label || f.rid}</span>
                {!f.missing && (
                  <CopyBtn value={f.name || f.label} onCopy={copy} label="کپی زیردامنه" />
                )}
                {f.missing && <Badge variant="error">حذف‌شده</Badge>}
              </div>
              <div className="cc-sub cc-copyline">
                <span>{f.zone || ''}</span>
                {f.content && (
                  <>
                    <span>·</span>
                    <span className="cc-mono cc-copyval">{f.content}</span>
                    <CopyBtn value={f.content} onCopy={copy} label="کپی مقدار" size={14} />
                  </>
                )}
              </div>
            </div>
            {f.missing ? (
              <Button
                variant="secondary-destructive"
                size="sm"
                icon={Trash}
                onClick={(e) => {
                  e.stopPropagation()
                  applyFav(f.rid, false)
                  void post('/api/cf/favs', { rid: f.rid })
                }}
              >
                پاک کردن
              </Button>
            ) : (
              <>
                <FavStar fav onClick={() => void setFav(f.acc, f.zid, f.rid, false)} label="حذف از منتخب" />
                <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
              </>
            )}
          </LayerCard>
        ))}
      </>
    )
  }

  function searchPage() {
    return (
      <>
        <SearchInput
          autoFocus
          placeholder="نام رکورد، آی‌پی یا دامنه…"
          value={q}
          onChange={setQ}
        />
        {!search && q.trim().length < 2 && <div className="cc-hint cc-center">حداقل ۲ حرف وارد کن.</div>}
        {searchErr && <ErrorState message={searchErr} onRetry={refresh} />}
        {!search && !searchErr && q.trim().length >= 2 && searchBusy && <SkeletonList />}
        {search && !searchErr && (
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
            {[...search.records]
              .sort((a, b) => Number(b.fav) - Number(a.fav))
              .map((r) => (
                <LayerCard
                  key={r.id}
                  className="cc-card cc-row cc-tap"
                  onClick={() => openInZone(r.acc, r.zid, r.zone, r.id)}
                >
                  <Badge variant="secondary" className={'cc-type cc-type-' + r.type.toLowerCase()}>{r.type}</Badge>
                  <div className="cc-grow">
                    <div className="cc-name cc-mono cc-copyline">
                      <span>{relName(r.name, r.zone)}</span>
                      <CopyBtn value={r.name} onCopy={copy} label="کپی زیردامنه" />
                    </div>
                    <div className="cc-sub cc-mono cc-copyline">
                      <span className="cc-copyval">{r.content}</span>
                      <CopyBtn value={r.content} onCopy={copy} label="کپی مقدار" size={14} />
                    </div>
                    <div className="cc-sub">
                      {r.zone}
                      {r.match === 'cname' ? ' · CNAME مرتبط با این IP' : ''}
                    </div>
                  </div>
                  <FavStar fav={r.fav} onClick={() => void setFav(r.acc, r.zid, r.id, !r.fav)} />
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
        {accs.length === 0 && !busy && !err && (
          <Empty title="اکانتی ثبت نشده" description="توکن کلادفلر را اضافه کن." />
        )}
        {accs.length === 0 && busy > 0 && <SkeletonList />}
        {accs.map((a) => (
          <LayerCard key={a.i} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">{a.name}</div>
              <div className="cc-sub">{(a.zones ?? 0) >= 0 ? num(a.zones) + ' دامنه' : 'خطا در خواندن دامنه‌ها'}</div>
            </div>
            <Button
              variant="secondary-destructive"
              size="sm"
              icon={Trash}
              aria-label="حذف اکانت"
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
              title: 'افزودن اکانت کلادفلر',
              desc: 'توکن باید دسترسی Zone → DNS → Edit و Email Routing داشته باشد.',
              fields: [
                {
                  key: 'name',
                  label: 'نام اکانت',
                  placeholder: 'مثلاً main',
                  validate: (v) => (v.trim() ? undefined : 'نام اکانت را وارد کن.'),
                },
                {
                  key: 'token',
                  label: 'API Token',
                  placeholder: 'توکن کلادفلر',
                  password: true,
                  validate: (v) => (v.trim().length >= 20 ? undefined : 'توکن معتبر نیست.'),
                },
              ],
              okText: 'افزودن',
              onSubmit: async (v) => {
                await post('/api/cf/accounts', { op: 'add', name: v.name, token: v.token })
              },
            })
          }
        >
          افزودن اکانت
        </Button>
      </>
    )
  }

  /* ---------------- zone ---------------- */

  function recordRow(r: CFRec) {
    if (view.v !== 'zone') return null
    const checked = sel.has(r.id)
    return (
      <LayerCard
        key={r.id}
        id={'rec-' + r.id}
        className={'cc-card cc-row cc-tap' + (selMode ? ' cc-selecting' : '') + (newId === r.id ? ' cc-new' : '')}
        role={selMode ? 'checkbox' : 'button'}
        aria-checked={selMode ? checked : undefined}
        aria-label={relName(r.name, zone?.name || '') + ' ' + r.type + ' ' + r.content}
        tabIndex={0}
        onClick={() => {
          if (view.v !== 'zone') return
          haptic('light')
          if (selMode) {
            const s = new Set(sel)
            if (checked) s.delete(r.id)
            else s.add(r.id)
            setSel(s)
          } else {
            setDlg({ k: 'record', acc: view.acc, zid: view.zid, zone: zone?.name || '', rec: r })
          }
        }}
        onKeyDown={(e: any) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            ;(e.currentTarget as HTMLElement).click()
          }
        }}
      >
        {selMode ? (
          <span className={'cc-check' + (checked ? ' on' : '')} aria-hidden="true">
            {checked ? '✓' : ''}
          </span>
        ) : (
          <Badge
            variant={PROXYABLE.includes(r.type) && r.proxied ? 'success' : 'secondary'}
            className={PROXYABLE.includes(r.type) && r.proxied ? undefined : 'cc-type cc-type-' + r.type.toLowerCase()}
          >
            {r.type}
          </Badge>
        )}
        <div className="cc-grow cc-recrow">
          <span className="cc-name cc-mono">{relName(r.name, zone?.name || '')}</span>
          {PROXYABLE.includes(r.type) && (
            <span
              className={'cc-px' + (r.proxied ? ' on' : '')}
              title={r.proxied ? 'پروکسی روشن' : 'پروکسی خاموش'}
            />
          )}
          <span className="cc-ip cc-mono">{r.content}</span>
          {!selMode && <CopyBtn value={r.content} onCopy={copy} label="کپی مقدار" size={13} />}
        </div>
        {!selMode && <FavStar fav={r.fav} onClick={() => void setFav(view.acc, view.zid, r.id, !r.fav)} />}
        {!selMode && <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>}
      </LayerCard>
    )
  }

  function recordsTab() {
    if (view.v !== 'zone') return null
    const ql = recQ.trim().toLowerCase()
    const list = records.filter(
      (r) =>
        (recType === 'all' || r.type === recType) &&
        (!ql ||
          r.name.toLowerCase().includes(ql) ||
          r.content.toLowerCase().includes(ql) ||
          r.type.toLowerCase() === ql),
    )
    const { favs: favList, rest } = splitFavorites(list)
    const types = Array.from(new Set(records.map((r) => r.type)))
    function selectAll() {
      if (sel.size === list.length) setSel(new Set())
      else setSel(new Set(list.map((r) => r.id)))
    }
    return (
      <>
        <div className="cc-grid2 cc-mb">
          <Button
            variant="primary"
            icon={Plus}
            onClick={() => setDlg({ k: 'editor', acc: view.acc, zid: view.zid, zone: zone?.name || '' })}
          >
            رکورد
          </Button>
          <Button
            variant={selMode ? 'secondary' : 'outline'}
            icon={selMode ? X : CheckSquare}
            onClick={() => {
              setSelMode(!selMode)
              setSel(new Set())
            }}
          >
            {selMode ? 'پایان' : 'انتخاب گروهی'}
          </Button>
        </div>
        {selMode && (
          <div className="cc-bulkbar" role="toolbar" aria-label="عملیات گروهی">
            <span className="cc-bulkcount">{faNum(sel.size)} انتخاب</span>
            <button className="cc-bulkbtn" onClick={selectAll} disabled={list.length === 0}>
              {sel.size === list.length && list.length > 0 ? <X size={14} /> : <CheckSquare size={14} />}
              {sel.size === list.length && list.length > 0 ? 'پاک‌کردن' : 'همه'}
            </button>
            <button className="cc-bulkbtn red" disabled={!sel.size} onClick={confirmBulkDelete}>
              <Trash size={14} /> حذف
            </button>
            <button className="cc-bulkbtn" disabled={!sel.size} onClick={bulkEdit}>
              <PencilSimple size={14} /> مقدار
            </button>
            <button className="cc-bulkbtn" disabled={!sel.size} onClick={bulkType}>
              <ArrowsClockwise size={14} /> نوع
            </button>
            <button className="cc-bulkbtn green" disabled={!sel.size} onClick={() => bulk('fav')}>
              <Star size={14} /> منتخب
            </button>
          </div>
        )}
        <SearchInput placeholder="جستجوی رکوردها" value={recQ} onChange={setRecQ} />
        {types.length > 1 && (
          <div className="cc-chips" role="group" aria-label="فیلتر نوع رکورد">
            {['all', ...types].map((t) => (
              <button
                key={t}
                className={'cc-chip' + (recType === t ? ' on' : '')}
                aria-pressed={recType === t}
                onClick={() => setRecType(t)}
              >
                {t === 'all' ? 'همه' : t}
              </button>
            ))}
          </div>
        )}
        <div className="cc-section">
          {num(list.length)} رکورد{sel.size > 0 ? ' · ' + faNum(sel.size) + ' انتخاب' : ''}
        </div>
        {records.length === 0 && busy > 0 && <SkeletonList />}
        {list.length === 0 && !busy && !err && <Empty title="رکوردی نیست" />}
        {favList.length > 0 && <div className="cc-section cc-section-fav">ستاره‌دار</div>}
        {favList.map(recordRow)}
        {favList.length > 0 && rest.length > 0 && <div className="cc-section">بقیه رکوردها</div>}
        {rest.map(recordRow)}
      </>
    )
  }

  function settingsTab() {
    if (view.v !== 'zone') return null
    if (!settings) return <SkeletonList rows={2} />
    return (
      <>
        <LayerCard className="cc-card">
          <div className="cc-kv">
            <span>حالت توسعه</span>
            <Switch
              checked={settings.dev}
              aria-label="حالت توسعه"
              onCheckedChange={() =>
                setDlg({
                  k: 'confirm',
                  title: settings.dev ? 'خاموش کردن حالت توسعه' : 'روشن کردن حالت توسعه',
                  desc: 'حالت توسعه کش را دور می‌زند و برای تست مناسب است.',
                  onOk: async () => {
                    if (view.v === 'zone')
                      await post('/api/cf/settings', { op: 'dev', acc: view.acc, zid: view.zid })
                  },
                })
              }
            />
          </div>
          <div className="cc-kv">
            <span>کش دامنه</span>
            <Button
              size="sm"
              variant="secondary-destructive"
              icon={Broom}
              onClick={() =>
                setDlg({
                  k: 'confirm',
                  title: 'پاک‌سازی کش',
                  desc: 'کل کش دامنه پاک شود؟',
                  onOk: async () => {
                    if (view.v === 'zone')
                      await post('/api/cf/settings', { op: 'purge', acc: view.acc, zid: view.zid })
                  },
                })
              }
            >
              پاک‌سازی
            </Button>
          </div>
        </LayerCard>
        {settings.groups.map((g) => (
          <div key={g.key}>
            <div className="cc-section">{g.title}</div>
            <LayerCard className="cc-card">
              {g.items.map((it) => (
                <div className="cc-kv" key={it.key}>
                  <span>{it.label}</span>
                  {it.kind === 'bool' ? (
                    <Switch
                      checked={it.value === 'on'}
                      aria-label={it.label}
                      onCheckedChange={(on) => applySetting(it, on ? 'on' : 'off')}
                    />
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      icon={CaretDown}
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
                            applySetting(it, v)
                          },
                        })
                      }
                    >
                      {it.values?.find((v) => v.v === it.value)?.l || it.value}
                    </Button>
                  )}
                </div>
              ))}
            </LayerCard>
          </div>
        ))}
        <div className="cc-grid2 cc-mt">
          <Button variant="outline" icon={Info} onClick={openZoneDetails}>
            جزئیات دامنه
          </Button>
          <Button
            variant={settings.paused ? 'primary' : 'secondary-destructive'}
            icon={settings.paused ? Play : Pause}
            onClick={() =>
              setDlg({
                k: 'confirm',
                title: settings.paused ? 'فعال‌سازی دامنه' : 'توقف دامنه',
                desc: settings.paused
                  ? 'دامنه دوباره روی کلادفلر فعال شود؟'
                  : 'دامنه متوقف شود؟ سایت از دسترس خارج می‌شود.',
                danger: !settings.paused,
                onOk: async () => {
                  if (view.v === 'zone')
                    await post('/api/cf/settings', { op: 'pause', acc: view.acc, zid: view.zid })
                },
              })
            }
          >
            {settings.paused ? 'فعال‌سازی دامنه' : 'توقف دامنه'}
          </Button>
        </div>
      </>
    )
  }

  function emailTab() {
    if (view.v !== 'zone') return null
    if (!email) return <SkeletonList rows={2} />
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
            <div className="cc-name">ایمیل فوروارد</div>
            <div className="cc-sub">
              با فعال‌سازی، رکوردهای MX/SPF خودکار ساخته می‌شوند. اگر سرویس ایمیل دیگری داری، خراب می‌شود.
            </div>
          </LayerCard>
          <Button
            className="cc-wide cc-green"
            icon={EnvelopeSimple}
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
            فعال‌سازی ایمیل
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
              variant="secondary-destructive"
              size="sm"
              icon={Trash}
              aria-label="حذف قانون"
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
            />

          </LayerCard>
        ))}
        <Button
          className="cc-wide cc-green cc-mb"
          icon={Plus}
          onClick={() =>
            setDlg({
              k: 'form',
              title: 'آدرس جدید',
              desc: 'آدرس محلی (مثلاً info) و مقصد تأییدشده را وارد کن.',
              fields: [
                {
                  key: 'addr',
                  label: 'آدرس',
                  placeholder: 'info',
                  hint: 'فقط بخش قبل از @ را وارد کن.',
                  validate: (v) =>
                    /^[a-z0-9._%+-]+$/i.test(v.trim()) ? undefined : 'آدرس محلی معتبر نیست.',
                },
                {
                  key: 'dest',
                  label: 'مقصد',
                  select: verified.length
                    ? verified.map((d) => ({ value: d.email, label: d.email }))
                    : undefined,
                  placeholder: 'you@gmail.com',
                  validate: (v) => (isEmail(v.trim()) ? undefined : 'ایمیل مقصد معتبر نیست.'),
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
          آدرس فوروارد
        </Button>

        <div className="cc-section">catch-all</div>
        <LayerCard className="cc-card cc-row">
          <div className="cc-grow">
            <div className="cc-sub">ایمیل‌های بدون قانون</div>
            <div className="cc-name">{email.catchAll || '—'}</div>
          </div>
          <Button variant="outline" size="sm" onClick={openCatchAll}>
            تنظیم
          </Button>
        </LayerCard>

        <div className="cc-section">مقصدها ({num((email.dests || []).length)})</div>
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
          icon={Plus}
          onClick={() =>
            setDlg({
              k: 'form',
              title: 'افزودن مقصد',
              desc: 'لینک تأیید به این ایمیل می‌رود.',
              fields: [
                {
                  key: 'dest',
                  label: 'ایمیل مقصد',
                  placeholder: 'you@gmail.com',
                  validate: (v) => (isEmail(v.trim()) ? undefined : 'ایمیل معتبر نیست.'),
                },
              ],
              okText: 'افزودن',
              onSubmit: async (v) => {
                if (view.v === 'zone')
                  await post('/api/cf/email', { op: 'dest_add', acc: view.acc, zid: view.zid, dest: v.dest })
              },
            })
          }
        >
          مقصد جدید
        </Button>
      </>
    )
  }

  function trafficTab() {
    return (
      <>
        <div className="cc-chips cc-mb" role="group" aria-label="بازه ترافیک">
          {[1, 7, 30].map((d) => (
            <button
              key={d}
              className={'cc-chip' + (days === d ? ' on' : '')}
              aria-pressed={days === d}
              onClick={() => setDays(d)}
            >
              {d === 1 ? 'امروز' : d === 7 ? '۷ روز' : '۳۰ روز'}
            </button>
          ))}
        </div>
        {!traffic ? (
          <SkeletonList rows={2} />
        ) : (
          <>
            <LayerCard className="cc-card">
              <div className="cc-name">مجموع کوئری DNS</div>
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
                  <div className="cc-name cc-mono cc-copyline">
                    <span>{relName(r.name, zone?.name || '')}</span>
                    <CopyBtn value={r.name} onCopy={copy} label="کپی زیردامنه" />
                  </div>
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
    if (!zone) return <SkeletonList rows={2} />
    return (
      <>
        <LayerCard className="cc-card cc-row">
          <div className="cc-grow">
            <div className="cc-name cc-mono">{zone?.name || ''}</div>
            <div className="cc-sub">
              {zone?.account || ''} {zone?.plan ? ' · ' + zone.plan : ''}
            </div>
          </div>
          <Badge variant={ZONE_BADGE[zone?.status || ''] || 'secondary'} dot>
            {statusLabel(zone?.status || '')}
          </Badge>
        </LayerCard>
        <div className="cc-tabs" role="tablist" aria-label="بخش‌های دامنه">
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
              role="tab"
              id={'tab-' + k}
              aria-selected={tab === k}
              aria-controls={'panel-' + k}
              tabIndex={tab === k ? 0 : -1}
              className={'cc-tab' + (tab === k ? ' on' : '')}
              onClick={() => openTab(k)}
            >
              {l}
            </button>
          ))}
        </div>
        <div role="tabpanel" id={'panel-' + tab} aria-labelledby={'tab-' + tab}>
          {tab === 'records' && recordsTab()}
          {tab === 'settings' && settingsTab()}
          {tab === 'email' && emailTab()}
          {tab === 'traffic' && trafficTab()}
        </div>
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
        {lbGroups.length === 0 && busy > 0 && <SkeletonList />}
        {lbGroups.length === 0 && !busy && !err && (
          <Empty title="گروهی نیست" description="رکورد A/AAAA/CNAME بساز تا گروه تشکیل شود." />
        )}
        {lbGroups.map((g) => (
          <LayerCard
            key={g.name}
            className="cc-card cc-row cc-tap"
            onClick={() => push({ v: 'lbgroup', acc: view.acc, zid: view.zid, name: g.name })}
          >
            <div className="cc-grow">
              <div className="cc-name cc-mono cc-copyline">
                <span>{relName(g.name, zone?.name || '')}</span>
                <CopyBtn value={g.name} onCopy={copy} label="کپی زیردامنه" />
              </div>
              <div className="cc-sub">
                {faNum(g.on)} فعال · {faNum(g.off)} غیرفعال
              </div>
            </div>
            <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
          </LayerCard>
        ))}
      </>
    )
  }

  function lbGroupPage() {
    if (view.v !== 'lbgroup') return null
    function entryAction(op: string, extra: any = {}, apply?: () => void, undo?: () => void) {
      if (view.v !== 'lbgroup') return
      setDlg(null)
      apply?.()
      void post('/api/cf/lb', { op, acc: view.acc, zid: view.zid, name: view.name, ...extra }, true, {
        refresh: false,
      }).then((d) => {
        if (!d) undo?.()
      })
    }
    function confirmProxy(on: boolean) {
      setDlg({
        k: 'confirm',
        title: on ? 'روشن‌کردن پروکسی همه' : 'خاموش‌کردن پروکسی همه',
        desc: `پروکسی برای هر ${faNum(lbEntries.length)} آی‌پی این گروه ${on ? 'روشن' : 'خاموش'} شود؟`,
        danger: !on,
        okText: on ? 'روشن کن' : 'خاموش کن',
        onOk: () => entryAction('proxy', { on }),
      })
    }
    return (
      <>
        <div className="cc-hintbox">
          <Info size={17} />
          <span>
            در این گروه، زاپاس فقط وقتی آی‌پی اصلی از کار بیفتد استفاده می‌شود؛ «جایگزینی» زاپاس را به‌جای آی‌پی اصلی
            می‌نشاند.
          </span>
        </div>
        <div className="cc-lb-top">
          <Button
            variant="primary"
            icon={Plus}
            onClick={() =>
              setDlg({
                k: 'form',
                title: 'افزودن آی‌پی',
                fields: [
                  {
                    key: 'type',
                    label: 'نوع',
                    select: ['A', 'AAAA', 'CNAME'].map((t) => ({ value: t, label: t })),
                  },
                  {
                    key: 'content',
                    label: 'مقدار',
                    placeholder: '1.2.3.4 یا host.example.com',
                    validate: (v) =>
                      isIPv4(v.trim()) || isIPv6(v.trim()) || isHost(v.trim())
                        ? undefined
                        : 'آی‌پی یا میزبان معتبر نیست.',
                  },
                ],
                okText: 'افزودن',
                onSubmit: async (v) => {
                  if (view.v !== 'lbgroup') return
                  const temp: LbEntry = {
                    on: true,
                    id: '',
                    type: v.type,
                    content: v.content,
                    proxied: false,
                    ttl: 1,
                    weight: 1,
                  }
                  setDlg(null)
                  addLb(temp)
                  const d = await post(
                    '/api/cf/lb',
                    {
                      op: 'add',
                      acc: view.acc,
                      zid: view.zid,
                      name: view.name,
                      type: v.type,
                      content: v.content,
                    },
                    true,
                    { refresh: false },
                  )
                  if (!d) setLbEntries((list) => list.filter((e) => e !== temp))
                },
              })
            }
          >
            افزودن آی‌پی
          </Button>
          <Button variant="secondary" icon={Globe} onClick={() => confirmProxy(true)}>
            پروکسی همه
          </Button>
        </div>
        <div className="cc-lb-links">
          <button type="button" className="cc-link" onClick={() => confirmProxy(false)}>
            خاموش‌کردن پروکسی همه
          </button>
          <span aria-hidden="true">·</span>
          <button
            type="button"
            className="cc-link"
            onClick={() => push({ v: 'lb', acc: view.acc, zid: view.zid })}
          >
            <ArrowRight size={13} />
            بازگشت به گروه‌ها
          </button>
        </div>
        <div className="cc-section">{faNum(lbEntries.length)} آی‌پی</div>
        {lbEntries.length === 0 && busy > 0 && <SkeletonList />}
        {lbEntries.length === 0 && !busy && !err && <Empty title="آی‌پی‌ای نیست" />}
        {lbEntries.map((e) => (
          <LayerCard key={e.content + e.type} className="cc-card cc-lb-card">
            <div className="cc-row">
              <span className={'cc-dot ' + (e.on ? 'run' : 'off')} />
              <div className="cc-grow">
                <div className="cc-name cc-mono cc-copyline">
                  <span>{e.content}</span>
                  <CopyBtn value={e.content} onCopy={copy} label="کپی آی‌پی" />
                </div>
                <div className="cc-sub cc-copyline">
                  <span>{e.type} · وزن {faNum(e.weight)}</span>
                  {e.spare ? (
                    <>
                      <span>· زاپاس:</span>
                      <span className="cc-mono cc-copyval">{e.spare.content}</span>
                      <CopyBtn value={e.spare.content} onCopy={copy} label="کپی آی‌پی زاپاس" size={14} />
                    </>
                  ) : (
                    <span>· بدون زاپاس</span>
                  )}
                </div>
              </div>
              <Switch
                checked={e.on}
                aria-label={'فعال بودن ' + e.content}
                onCheckedChange={() => {
                  const next = !e.on
                  entryAction(
                    'toggle',
                    { content: e.content, on: next },
                    () => patchLb(e.content, { on: next }),
                    () => patchLb(e.content, { on: e.on }),
                  )
                }}
              />
            </div>
            <div className="cc-lb-actions">
              <button
                type="button"
                className="cc-lb-act"
                onClick={() =>
                  setDlg({
                    k: 'form',
                    title: 'وزن آی‌پی',
                    fields: [
                      {
                        key: 'weight',
                        label: 'وزن (۰ یا بیشتر)',
                        value: String(e.weight),
                        validate: (v) => {
                          const n = Number(v)
                          return Number.isFinite(n) && n >= 0 ? undefined : 'وزن باید عددی صفر یا بیشتر باشد.'
                        },
                      },
                    ],
                    okText: 'ذخیره',
                    onSubmit: async (v) => {
                      if (view.v !== 'lbgroup') return
                      const prev = e.weight
                      const next = Number(v.weight || 0)
                      patchLb(e.content, { weight: next })
                      setDlg(null)
                      const d = await post(
                        '/api/cf/lb',
                        {
                          op: 'weight',
                          acc: view.acc,
                          zid: view.zid,
                          name: view.name,
                          content: e.content,
                          weight: next,
                        },
                        true,
                        { refresh: false },
                      )
                      if (!d) patchLb(e.content, { weight: prev })
                    },
                  })
                }
              >
                <SlidersHorizontal size={16} />
                <span>وزن</span>
                <b>{faNum(e.weight)}</b>
              </button>
              <button
                type="button"
                className="cc-lb-act"
                onClick={() =>
                  setDlg({ k: 'spare', acc: view.acc, zid: view.zid, name: view.name, entry: e })
                }
              >
                <Lifebuoy size={16} />
                <span>زاپاس</span>
                <b className={e.spare ? 'cc-lb-ok' : 'cc-lb-muted'}>{e.spare ? 'فعال' : 'ندارد'}</b>
              </button>
              {e.spare && (
                <button
                  type="button"
                  className="cc-lb-act"
                  onClick={() =>
                    setDlg({
                      k: 'confirm',
                      title: 'جایگزینی با زاپاس',
                      desc: `آی‌پی زاپاس ${e.spare?.content} جای ${e.content} بنشیند؟`,
                      onOk: () => {
                        const sp = e.spare
                        if (!sp) return
                        const nextType = sp.type || e.type
                        entryAction(
                          'replace',
                          { content: e.content },
                          () => patchLb(e.content, { content: sp.content, type: nextType, spare: undefined }),
                          () => patchLb(sp.content, { content: e.content, type: e.type, spare: e.spare }),
                        )
                      },
                    })
                  }
                >
                  <ArrowsClockwise size={16} />
                  <span>جایگزینی</span>
                </button>
              )}
              <button
                type="button"
                className="cc-lb-del"
                aria-label={'حذف آی‌پی ' + e.content}
                onClick={() =>
                  setDlg({
                    k: 'confirm',
                    title: 'حذف آی‌پی از گروه',
                    desc: `${e.type} · ${e.content} و تنظیمات وزن/زاپاس آن حذف شود؟ این کار رکورد DNS را هم پاک می‌کند.`,
                    danger: true,
                    okText: 'حذف',
                    onOk: () =>
                      entryAction(
                        'del',
                        { content: e.content },
                        () => dropLb(e.content),
                        () => addLb(e),
                      ),
                  })
                }
              >
                <Trash size={16} />
              </button>
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
      <Dialog.Root
        open
        onOpenChange={(o: boolean) => {
          if (o) return
          setDlg(null)
          leaveSearchDialog()
        }}
      >
        <Dialog className={"cc-dialog" + (dlg && dlg.k === 'record' ? ' cc-recsheet' : '')} size="base">
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
                    <span className="cc-chev"><CaretLeft size={15} weight="bold" /></span>
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
          {dlg.k === 'tree' && <RefTreeDialog dlg={dlg} onClose={() => setDlg(null)} onCopy={copy} />}
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
                    if (view.v !== 'zone') return
                    const rec = dlg.rec
                    setDlg(null)
                    dropRec(rec.id)
                    const d = await post(
                      '/api/cf/record',
                      { op: 'del', acc: dlg.acc, zid: dlg.zid, rid: rec.id },
                      true,
                      { refresh: false },
                    )
                    if (!d) refresh()
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
                const prev = dlg.rec
                let temp: CFRec | null = null
                if (prev) {
                  patchRec({
                    ...prev,
                    content: payload.content,
                    ttl: payload.ttl,
                    proxied: payload.proxied,
                    priority: payload.priority,
                  })
                } else {
                  const name = fqdn(payload.name, dlg.zone)
                  temp = {
                    id: 'tmp-' + Date.now(),
                    type: payload.type,
                    name,
                    content: payload.content,
                    ttl: payload.ttl,
                    proxied: payload.proxied,
                    priority: payload.priority,
                    label: relName(name, dlg.zone) + '-' + payload.type,
                    fav: false,
                  }
                  addRec(temp)
                  setNewId(temp.id)
                }
                setDlg(null)
                const d = await post(path, body, true, { refresh: false })
                if (d && d.record) {
                  if (prev) patchRec(d.record)
                  else if (temp) {
                    const tid = temp.id
                    setRecords((list) => list.map((r) => (r.id === tid ? d.record : r)))
                    if (d.record.id) setNewId(d.record.id)
                  }
                } else if (!d) {
                  if (prev) patchRec(prev)
                  else if (temp) {
                    const tid = temp.id
                    setRecords((list) => list.filter((r) => r.id !== tid))
                  }
                }
              }}
            />
          )}
          {dlg.k === 'spare' && (
            <SpareSheet
              dlg={dlg}
              onClose={() => setDlg(null)}
              onCopy={copy}
              onSet={(spare) => {
                if (view.v !== 'lbgroup') return
                const entry = dlg.entry
                const prev = entry.spare
                const type = isIPv4(spare) ? 'A' : isIPv6(spare) ? 'AAAA' : 'CNAME'
                patchLb(entry.content, { spare: { type, content: spare } })
                setDlg(null)
                void post(
                  '/api/cf/lb',
                  {
                    op: 'spare_set',
                    acc: dlg.acc,
                    zid: dlg.zid,
                    name: dlg.name,
                    content: entry.content,
                    spare,
                  },
                  true,
                  { refresh: false },
                ).then((d) => {
                  if (!d) patchLb(entry.content, { spare: prev })
                })
              }}
              onDel={() => {
                const s = dlg
                setDlg({
                  k: 'confirm',
                  title: 'حذف زاپاس',
                  desc: `زاپاس ${s.entry.spare?.content || ''} برای ${s.entry.content} حذف شود؟`,
                  danger: true,
                  okText: 'حذف',
                  onOk: async () => {
                    const prev = s.entry.spare
                    setDlg(null)
                    patchLb(s.entry.content, { spare: undefined })
                    const d = await post(
                      '/api/cf/lb',
                      {
                        op: 'spare_del',
                        acc: s.acc,
                        zid: s.zid,
                        name: s.name,
                        content: s.entry.content,
                      },
                      true,
                      { refresh: false },
                    )
                    if (!d) patchLb(s.entry.content, { spare: prev })
                  },
                })
              }}
            />
          )}
        </Dialog>
      </Dialog.Root>
    )
  }

  /* ---------------- render ---------------- */

  let body: any = null
  if (err) body = <ErrorState message={err} onRetry={refresh} />
  else if (view.v === 'zones') body = zonesPage()
  else if (view.v === 'accounts') body = accountsPage()
  else if (view.v === 'favs') body = favsPage()
  else if (view.v === 'search') body = searchPage()
  else if (view.v === 'zone') body = zonePage()
  else if (view.v === 'lbzones') body = lbZonesPage()
  else if (view.v === 'lb') body = lbPage()
  else if (view.v === 'lbgroup') body = lbGroupPage()

  const root = stack.length === 0 && view.v !== 'zones'

  function goRoot(v: View) {
    setStack([])
    setView(v)
  }
  const cfSections: SubItem[] = [
    { id: 'zones', label: 'زون‌ها', active: view.v === 'zones', onSelect: () => goRoot({ v: 'zones' }) },
    { id: 'accounts', label: 'اکانت‌ها', active: view.v === 'accounts', onSelect: () => goRoot({ v: 'accounts' }) },
    { id: 'lbzones', label: 'لودبالانسر', active: view.v === 'lbzones', onSelect: () => goRoot({ v: 'lbzones' }) },
    { id: 'favs', label: 'موردعلاقه', active: view.v === 'favs', onSelect: () => goRoot({ v: 'favs' }) },
  ]

  return (
    <>
      <ProviderShell
        onTool={onTool}
        provider="cf"
        sections={cfSections}
        title={title}
        busy={busy > 0}
        showBack={root || stack.length > 0}
        showSwitch={view.v === 'zones' && stack.length === 0}
        onBack={goBack}
        onRefresh={refresh}
        onChange={onSwitch}
        toast={toastMsg}
      >
        {body}
      </ProviderShell>
      {renderDlg()}
    </>
  )
}
