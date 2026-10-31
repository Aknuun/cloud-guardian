import { useEffect, useMemo, useState } from 'react'
import { ArrowClockwise, Check, GlobeSimple, PencilSimple, Pulse, ShieldCheck, SquaresFour, Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from './ui'
import { FilterBar } from './FilterBar'
import { ErrBox, FormDialog, Skeleton } from './toolsMeta'
import { StatGrid } from './Stats'

type Row = {
  acc: number
  zid: string
  zone: string
  rid: string
  name: string
  type: string
  content: string
  proxied: boolean
  status: string
  detail: string
  latency: number
  http: number
  tlsDays: number
}

type State = {
  enabled: boolean
  intervalMin: number
  lastRun: number
  accounts: string[]
  last: { at: number; rows: Row[]; total: number; bad: number }
}

const STATUS_FA: Record<string, string> = {
  ok: 'سالم',
  proxy: 'پشت کلادفلر',
  nxdomain: 'ریسالو نمی‌شود',
  timeout: 'پاسخ نمی‌دهد',
  bad: 'مقدار نامعتبر',
}

export function CfCheckView() {
  const [st, setSt] = useState<State | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')
  const [fix, setFix] = useState<Row | null>(null)
  const [fixVal, setFixVal] = useState('')
  const [fixType, setFixType] = useState('A')
  const [fixErr, setFixErr] = useState('')
  const [del, setDel] = useState<Row | null>(null)
  const [cfg, setCfg] = useState<{ enabled: boolean; intervalMin: number } | null>(null)
  const [notice, setNotice] = useState('')
  const [zones, setZones] = useState<{ acc: number; aname: string; zid: string; zone: string }[] | null>(null)
  const [zpick, setZpick] = useState<Record<string, boolean>>({})
  const [zoneOpen, setZoneOpen] = useState(false)
  const [sel, setSel] = useState<string[]>([])
  const [bulk, setBulk] = useState<'edit' | 'del' | 'bad' | null>(null)
  const [bulkVal, setBulkVal] = useState('')
  const [bulkErr, setBulkErr] = useState('')

  const load = async () => {
    try {
      setErr('')
      setSt(await api<State>('/api/tools/cfcheck?t=' + Date.now()))
    } catch (e: any) {
      setErr(e.message || 'دریافت وضعیت ناموفق')
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const run = async (scope: 'all' | Row = 'all') => {
    setBusy(true)
    setNotice('')
    try {
      const body = scope === 'all' ? {} : { acc: scope.acc, zid: scope.zid, rid: scope.rid }
      const res = await api<{ at: number; rows: Row[]; total: number; bad: number }>('/api/tools/cfcheck/run', body)
      setSt((s) => (s ? { ...s, last: res, lastRun: res.at } : s))
      setNotice(`بررسی انجام شد: ${faNum(res.total)} رکورد، ${faNum(res.bad)} مشکل`)
      invalidateCache()
    } catch (e: any) {
      setNotice(e.message || 'بررسی ناموفق بود')
    }
    setBusy(false)
  }

  const rows = st?.last?.rows || []
  const key = (r: Row) => r.acc + '|' + r.zid + '|' + r.rid

  const openZones = async () => {
    setZoneOpen(true)
    if (zones) return
    try {
      const d = await api<{ zones: any[]; accounts: { i: number; name: string }[] }>('/api/cf/zones?t=' + Date.now())
      const names: Record<number, string> = {}
      for (const a of d.accounts || []) names[a.i] = a.name
      setZones(
        (d.zones || []).map((z: any) => ({ acc: z.acc, aname: names[z.acc] || 'اکانت ' + z.acc, zid: z.id, zone: z.name })),
      )
    } catch (e: any) {
      setNotice(e.message || 'لیست دامنه‌ها نیامد')
    }
  }

  const runZones = async () => {
    const zids = (zones || []).filter((z) => zpick[z.acc + '|' + z.zid]).map((z) => z.zid)
    if (!zids.length) {
      setNotice('حداقل یک دامنه انتخاب کن')
      return
    }
    setZoneOpen(false)
    setBusy(true)
    setNotice('')
    try {
      const res = await api<{ at: number; rows: Row[]; total: number; bad: number }>('/api/tools/cfcheck/run', { zids })
      setSt((s2) => (s2 ? { ...s2, last: res, lastRun: res.at } : s2))
      setNotice(`بررسی انجام شد: ${faNum(res.total)} رکورد، ${faNum(res.bad)} مشکل`)
      invalidateCache()
    } catch (e: any) {
      setNotice(e.message || 'بررسی ناموفق بود')
    }
    setBusy(false)
  }

  const bad = rows.filter((r) => r.status !== 'ok' && r.status !== 'proxy')
  const picked = rows.filter((r) => sel.includes(key(r)))

  const bulkApply = async () => {
    const items = (bulk === 'bad' ? bad : picked).map((r) => ({ acc: r.acc, zid: r.zid, rid: r.rid }))
    if (!items.length) {
      setBulkErr('رکوردی انتخاب نشده')
      return
    }
    setBulkErr('')
    try {
      const res = await api<{ ok: number; fail: number }>('/api/tools/cfcheck/bulk', {
        op: bulk === 'del' ? 'del' : 'edit',
        content: bulk === 'del' ? '' : bulkVal.trim(),
        items,
      })
      setBulk(null)
      setSel([])
      invalidateCache()
      await load()
      setNotice(`انجام شد: ${faNum(res.ok)} موفق، ${faNum(res.fail)} ناموفق`)
    } catch (e: any) {
      setBulkErr(e.message || 'انجام نشد')
    }
  }

  const runBadAgain = async () => {
    const zids = [...new Set(bad.map((r) => r.zid))]
    setBulk(null)
    if (!zids.length) {
      setNotice('موردی برای بررسی نیست')
      return
    }
    setBusy(true)
    try {
      const res = await api<{ at: number; rows: Row[]; total: number; bad: number }>('/api/tools/cfcheck/run', { zids })
      setSt((s2) => (s2 ? { ...s2, last: res, lastRun: res.at } : s2))
      setNotice(`بررسی مجدد: ${faNum(res.bad)} مشکل`)
    } catch (e: any) {
      setNotice(e.message || 'بررسی ناموفق بود')
    }
    setBusy(false)
  }

  const shown = useMemo(() => {
    const ql = q.trim().toLowerCase()
    return rows.filter((r) => {
      if (flt === 'bad' && (r.status === 'ok' || r.status === 'proxy')) return false
      if (flt === 'ok' && r.status !== 'ok' && r.status !== 'proxy') return false
      if (flt === 'proxy' && r.status !== 'proxy') return false
      if (!ql) return true
      return (r.name + ' ' + r.content + ' ' + r.zone).toLowerCase().includes(ql)
    })
  }, [rows, q, flt])

  const groups = useMemo(() => {
    const m = new Map<string, Row[]>()
    for (const r of shown) {
      const k = r.zone
      if (!m.has(k)) m.set(k, [])
      m.get(k)!.push(r)
    }
    return [...m.entries()]
  }, [shown])

  const okCount = rows.filter((r) => r.status === 'ok' || r.status === 'proxy').length

  const saveFix = async () => {
    if (!fix) return
    setFixErr('')
    try {
      await api('/api/tools/cfcheck/fix', {
        acc: fix.acc,
        zid: fix.zid,
        rid: fix.rid,
        content: fixVal.trim(),
        type: fixType,
      })
      setFix(null)
      invalidateCache()
      await run(fix)
    } catch (e: any) {
      setFixErr(e.message || 'ذخیره نشد')
    }
  }

  const removeRow = async () => {
    if (!del) return
    try {
      await api('/api/tools/cfcheck/fix', { acc: del.acc, zid: del.zid, rid: del.rid, del: true })
      setDel(null)
      invalidateCache()
      await load()
    } catch (e: any) {
      setNotice(e.message || 'حذف نشد')
    }
  }

  const saveCfg = async () => {
    if (!cfg) return
    try {
      await api('/api/tools/cfcheck/cfg', cfg)
      setCfg(null)
      await load()
      setNotice(cfg.enabled ? 'مانیتور روشن شد' : 'مانیتور خاموش شد')
    } catch (e: any) {
      setNotice(e.message || 'ذخیره نشد')
    }
  }

  return (
    <>
      <StatGrid
        items={[
          { label: 'رکوردهای بررسی‌شده', value: faNum(st?.last?.total || 0) },
          { label: 'سالم', value: faNum(okCount), pct: st?.last?.total ? (okCount / st.last.total) * 100 : 0 },
          { label: 'مشکل‌دار', value: faNum(st?.last?.bad || 0) },
        ]}
      />

      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی نام یا مقدار"
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'bad', label: 'مشکل‌دار' },
          { id: 'ok', label: 'سالم' },
          { id: 'proxy', label: 'پشت کلادفلر' },
        ]}
        value={flt}
        onValue={setFlt}
        onRefresh={() => void load()}
      />

      <div className="cc-btnrow cc-mb">
        <Button variant="primary" icon={Pulse} onClick={() => void run('all')} disabled={busy}>
          {busy ? 'در حال بررسی…' : 'بررسی همه'}
        </Button>
        <Button variant="outline" icon={GlobeSimple} onClick={() => void openZones()}>
          انتخاب دامنه‌ها
        </Button>
        <Button variant="outline" icon={SquaresFour} onClick={() => { setBulkErr(''); setBulkVal(''); setBulk('bad') }}>
          عملیات گروهی
        </Button>
        <Button variant="outline" icon={ShieldCheck} onClick={() => setCfg({ enabled: !!st?.enabled, intervalMin: st?.intervalMin || 60 })}>
          مانیتور
        </Button>
        {st?.enabled ? (
          <span className="cc-sub">هر {faNum(st?.intervalMin || 60)} دقیقه · روشن</span>
        ) : null}
      </div>

      {sel.length > 0 && (
        <div className="cc-bulkbar">
          <span>{faNum(sel.length)} انتخاب شده</span>
          <Button variant="outline" onClick={() => { setBulkErr(''); setBulkVal(''); setBulk('edit') }}>
            ویرایش
          </Button>
          <Button variant="outline" onClick={() => { setBulkErr(''); setBulk('del') }}>
            حذف
          </Button>
          <Button variant="outline" onClick={() => setSel([])}>
            لغو
          </Button>
        </div>
      )}

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}
      {err && <ErrBox msg={err} onRetry={load} />}
      {busy && <div className="cc-center cc-pad"><Loader size="lg" /></div>}
      {!st && !busy && !err && <Skeleton rows={5} />}
      {st && rows.length === 0 && !busy && (
        <Empty
          title="هنوز بررسی نشده"
          description="با «بررسی همه» تمام رکوردهای A/AAAA/CNAME در همهٔ اکانت‌های کلادفلر تست می‌شوند."
        />
      )}

      {groups.map(([zone, list]) => (
        <div key={zone} className="cc-section">
          <div className="cc-section-head">
            <h2>{zone}</h2>
            <span className="cc-sub">{faNum(list.length)} رکورد</span>
          </div>
          {list.map((r) => {
            const bad = r.status !== 'ok' && r.status !== 'proxy'
            return (
              <div key={r.rid} className={'cc-trow' + (sel.includes(key(r)) ? ' sel' : '')}>
                <label className="cc-pick" onClick={(e) => e.stopPropagation()}>
                  <input
                    type="checkbox"
                    id={'cfpick-' + r.rid}
                    name={'cfpick'}
                    aria-label={'انتخاب ' + r.name}
                    checked={sel.includes(key(r))}
                    onChange={(e) =>
                      setSel((cur) => (e.target.checked ? [...cur, key(r)] : cur.filter((k) => k !== key(r))))
                    }
                  />
                </label>
                <span className={'cc-dot' + (bad ? ' hot' : r.status === 'proxy' ? ' pw' : '')} />
                <div className="cc-grow">
                  <div className="cc-name cc-ltr">{r.name}</div>
                  <div className="cc-sub cc-ltr">
                    {r.type} · {r.content}
                    {r.latency ? ' · ' + faNum(r.latency) + ' ms' : ''}
                    {r.http ? ' · HTTP ' + faNum(r.http) : ''}
                    {r.tlsDays ? ' · SSL ' + faNum(r.tlsDays) + ' روز' : ''}
                  </div>
                  <div className={'cc-sub' + (bad ? ' cc-hot' : '')}>
                    {STATUS_FA[r.status] || r.status}
                    {r.detail ? ' — ' + r.detail : ''}
                  </div>
                </div>
                <Badge variant={bad ? 'error' : r.status === 'proxy' ? 'secondary' : 'success'}>
                  {STATUS_FA[r.status] || r.status}
                </Badge>
                <div className="cc-rowbtns">
                  <button
                    type="button"
                    className="cc-rowbtn"
                    aria-label="بررسی دوباره"
                    onClick={() => void run(r)}
                  >
                    <ArrowClockwise size={15} />
                  </button>
                  <button
                    type="button"
                    className="cc-rowbtn"
                    aria-label="اصلاح رکورد"
                    onClick={() => {
                      setFixErr('')
                      setFix(r)
                      setFixVal(r.content)
                      setFixType(r.type)
                    }}
                  >
                    <PencilSimple size={15} />
                  </button>
                  <button type="button" className="cc-rowbtn danger" aria-label="حذف رکورد" onClick={() => setDel(r)}>
                    <Trash size={15} />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      ))}

      {zoneOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setZoneOpen(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">انتخاب دامنه‌ها</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              فقط رکوردهای A/AAAA/CNAME همین دامنه‌ها بررسی می‌شوند.
            </Dialog.Description>
            {!zones && <div className="cc-center cc-pad"><Loader /></div>}
            {zones && (
              <>
                <label className="cc-checkline">
                  <input
                    type="checkbox"
                    id="cfz-all"
                    name="cfzAll"
                    checked={zones.every((z) => zpick[z.acc + '|' + z.zid])}
                    onChange={(e) => {
                      const on = e.target.checked
                      const next: Record<string, boolean> = {}
                      for (const z of zones) next[z.acc + '|' + z.zid] = on
                      setZpick(next)
                    }}
                  />
                  <span className="cc-grow">همهٔ دامنه‌ها ({faNum(zones.length)})</span>
                </label>
                {zones.map((z) => (
                  <label key={z.acc + '|' + z.zid} className="cc-checkline">
                    <input
                      type="checkbox"
                      id={'cfz-' + z.zid}
                      name="cfz"
                      checked={!!zpick[z.acc + '|' + z.zid]}
                      onChange={(e) => setZpick((p) => ({ ...p, [z.acc + '|' + z.zid]: e.target.checked }))}
                    />
                    <span className="cc-grow cc-ltr">{z.zone}</span>
                    <span className="cc-sub">{z.aname}</span>
                  </label>
                ))}
              </>
            )}
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setZoneOpen(false)}>انصراف</Button>
              <Button variant="primary" icon={Pulse} onClick={() => void runZones()}>بررسی انتخاب‌شده‌ها</Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {bulk && (
        <Dialog.Root open onOpenChange={(o) => !o && setBulk(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">
              {bulk === 'del' ? 'حذف گروهی' : bulk === 'edit' ? 'ویرایش گروهی' : 'عملیات گروهی'}
            </Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              {bulk === 'bad'
                ? `${faNum(bad.length)} رکورد مشکل‌دار در آخرین بررسی`
                : `${faNum(picked.length)} رکورد انتخاب شده`}
            </Dialog.Description>
            {bulk === 'bad' ? (
              <div className="cc-form">
                <Button variant="outline" icon={ArrowClockwise} onClick={() => void runBadAgain()}>
                  بررسی مجدد خطادارها
                </Button>
                <Button variant="outline" onClick={() => setSel(bad.map(key))}>
                  انتخاب همهٔ خطادارها
                </Button>
                <Button variant="outline" onClick={() => { setBulk('edit'); setBulkVal('') }}>
                  ویرایش مقدار خطادارها
                </Button>
                <Button variant="outline" onClick={() => setBulk('del')}>
                  حذف همهٔ خطادارها
                </Button>
              </div>
            ) : (
              <div className="cc-form">
                {bulk === 'edit' && (
                  <label className="cc-field">
                    <span>مقدار جدید</span>
                    <Input className="cc-ltr" value={bulkVal} onChange={(e) => setBulkVal(e.target.value)} placeholder="1.2.3.4 یا host.example.com" />
                  </label>
                )}
                {bulk === 'del' && (
                  <div className="cc-hint">این رکوردها از کلادفلر حذف می‌شوند و برگشت‌پذیر نیست.</div>
                )}
                {bulkErr && <div className="cc-form-err">{bulkErr}</div>}
              </div>
            )}
            {(bulk === 'edit' || bulk === 'del') && (
              <div className="cc-dialog-actions">
                <Button variant="secondary" onClick={() => setBulk('bad')}>بازگشت</Button>
                <Button variant={bulk === 'del' ? 'destructive' : 'primary'} icon={bulk === 'del' ? Trash : Check} onClick={() => void bulkApply()}>
                  {bulk === 'del' ? 'حذف' : 'ذخیره'}
                </Button>
              </div>
            )}
          </Dialog>
        </Dialog.Root>
      )}

      {fix && (
        <FormDialog
          title="اصلاح رکورد"
          sub={fix.name}
          onClose={() => setFix(null)}
          error={fixErr}
          onSave={saveFix}
        >
          <label className="cc-field">
            <span>نوع</span>
            <select className="cc-input" value={fixType} onChange={(e) => setFixType(e.target.value)}>
              {['A', 'AAAA', 'CNAME'].map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </label>
          <label className="cc-field">
            <span>مقدار</span>
            <Input className="cc-ltr" value={fixVal} onChange={(e) => setFixVal(e.target.value)} placeholder="1.2.3.4 یا host.example.com" />
          </label>
        </FormDialog>
      )}

      {del && (
        <Dialog.Root open onOpenChange={(o) => !o && setDel(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف رکورد</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              «{del.name}» ({del.type}) از کلادفلر حذف می‌شود. این عمل بازگشت‌پذیر نیست.
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setDel(null)}>انصراف</Button>
              <Button variant="destructive" icon={Trash} onClick={removeRow}>حذف</Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {cfg && (
        <FormDialog
          title="مانیتور خودکار"
          sub="هر چند دقیقه همهٔ رکوردها بررسی و در صورت خرابی جدید، در تلگرام خبر داده می‌شود."
          onClose={() => setCfg(null)}
          onSave={saveCfg}
        >
          <label className="cc-checkline">
            <input
              type="checkbox"
              id="cfh-on"
              name="cfhOn"
              checked={cfg.enabled}
              onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })}
            />
            <span className="cc-grow">مانیتور روشن باشد</span>
          </label>
          <label className="cc-field">
            <span>فاصلهٔ بررسی (دقیقه)</span>
            <Input
              type="number"
              min={5}
              max={1440}
              value={cfg.intervalMin}
              onChange={(e) => setCfg({ ...cfg, intervalMin: Number(e.target.value) || 60 })}
            />
          </label>
          {st?.lastRun ? <div className="cc-hint">آخرین اجرا: {faDate(st.lastRun)}</div> : null}
        </FormDialog>
      )}
    </>
  )
}
