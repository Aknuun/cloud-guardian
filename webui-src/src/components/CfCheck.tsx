import { useEffect, useMemo, useState } from 'react'
import { ArrowClockwise, PencilSimple, Pulse, ShieldCheck, Trash } from '@phosphor-icons/react'
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
        <Button variant="outline" icon={ShieldCheck} onClick={() => setCfg({ enabled: !!st?.enabled, intervalMin: st?.intervalMin || 60 })}>
          مانیتور
        </Button>
        {st?.enabled ? (
          <Button variant="outline" onClick={() => setCfg({ enabled: true, intervalMin: st?.intervalMin || 60 })}>
            هر {faNum(st?.intervalMin || 60)} دقیقه · روشن
          </Button>
        ) : null}
      </div>

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
              <div key={r.rid} className={'cc-trow' + (bad ? ' sel' : '')}>
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
