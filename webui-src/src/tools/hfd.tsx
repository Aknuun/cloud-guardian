import { useEffect, useState } from 'react'
import { CaretLeft, Lightning, Envelope, GearSix, Lifebuoy, Broadcast, ArrowsClockwise, CalendarDots, BellSimple, Fire, GlobeSimple, Pulse, ArrowClockwise, BellRinging, Check, Copy, Gear, HardDrive, Info, PencilSimple, Plus, Power, ShieldCheck, Terminal, Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { StatGrid } from '../components/Stats'
import { FilterBar } from '../components/FilterBar'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, save, faTxt } from './shared'
import type { Srv, Rem, PgPanel, PgMon, PgData } from './shared'

type HfdCfg = { enabled: boolean; intervalH: number; at: string; nextRun: string; totalCreated: number }
type HfdData = { cfg: HfdCfg; panels: { idx: number; name: string }[]; items: { key: string; panel: number; host: string }[] }
type HfdHost = { id: string; remark: string; disabled: boolean; reality: boolean; fields: Record<string, boolean> }

function HfdView() {
  const { data, err, reload } = useTool<HfdData>('/api/tools/hfd')
  const [cfg, setCfg] = useState<HfdCfg | null>(null)
  const [pi, setPi] = useState(0)
  const [hosts, setHosts] = useState<HfdHost[] | null>(null)
  const [hErr, setHErr] = useState('')
  const [notice, setNotice] = useState('')
  const [fErr, setFErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [logOpen, setLogOpen] = useState(false)
  const [log, setLog] = useState<any[] | null>(null)
  const [revOpen, setRevOpen] = useState(false)
  const [revs, setRevs] = useState<any[] | null>(null)

  useEffect(() => {
    if (data?.cfg) setCfg(data.cfg)
  }, [data])
  useEffect(() => {
    if (!data?.panels?.length) return
    loadHosts(pi)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, pi])

  async function loadHosts(p: number) {
    setHErr('')
    try {
      const d = await api<{ hosts: HfdHost[] }>(`/api/tools/hfd/hosts?panel=${p}&t=${Date.now()}`)
      setHosts(d.hosts || [])
    } catch (e: any) {
      setHosts([])
      setHErr(e.message || 'خواندن هاست‌ها ناموفق')
    }
  }
  async function act(path: string, body: any, msg?: string) {
    setFErr('')
    try {
      await api(path, body)
      invalidateCache()
      if (msg) setNotice(msg)
      return true
    } catch (e: any) {
      setFErr(e.message || 'انجام نشد')
      return false
    }
  }
  if (err) return <Empty title="خطا" description={err} />
  if (!data || !cfg)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
    )
  const next = cfg.nextRun ? faDate(cfg.nextRun) : '—'
  return (
    <>
      <StatGrid
        items={[
          { label: 'وضعیت', value: cfg.enabled ? 'فعال' : 'خاموش' },
          { label: 'هر', value: faNum(cfg.intervalH) + ' ساعت' },
          { label: 'اجرای بعدی', value: next },
          { label: 'هاست‌های انتخابی', value: faNum(data.items.length) },
        ]}
      />
      <div className="cc-hint cc-mb">
        هر {faNum(cfg.intervalH)} ساعت، ساب‌های هاست‌های انتخاب‌شده با نام تازه بازسازی می‌شوند تا فیلترها دور زده شوند.
        برای هر هاست یکی از سه فیلد «آدرس» (IP درخواست)، «SNI» (نام TLS) یا «Host» (هدر دامنه) را روشن کن.
      </div>

      <div className="cc-form">
        <label className="cc-checkline">
          <input id="cgc8" name="cgc8" type="checkbox" checked={!!cfg.enabled} onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })} />
          <span>چرخش روزانه فعال باشد</span>
        </label>
        <div className="cc-field-row">
          <label className="cc-field">
            <span>فاصله (ساعت)</span>
            <Input
              type="number"
              value={String(cfg.intervalH)}
              onChange={(e) => setCfg({ ...cfg, intervalH: Number(e.target.value) || 0 })}
            />
          </label>
          <label className="cc-field">
            <span>ساعت شروع (HH:MM)</span>
            <Input className="cc-input" value={cfg.at} onChange={(e) => setCfg({ ...cfg, at: e.target.value })} placeholder="03:00" />
          </label>
        </div>
        {cfg.totalCreated > 0 && <div className="cc-hint">تاکنون {faNum(cfg.totalCreated)} ساب ساخته شده.</div>}
        {fErr && <div className="cc-form-err">{fErr}</div>}
        <Button
          variant="primary"
          onClick={async () => {
            if (await act('/api/tools/hfd/cfg', cfg, 'تنظیمات ذخیره شد')) reload()
          }}
        >
          ذخیرهٔ تنظیمات
        </Button>
      </div>

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>هاست‌ها</h2>
          <Button
            variant="outline"
            disabled={busy}
            onClick={async () => {
              setBusy(true)
              const ok = await act('/api/tools/hfd/run', {}, 'چرخش شروع شد؛ نتیجه در تلگرام می‌آید')
              setBusy(false)
              if (!ok) setNotice('')
            }}
          >
            {busy ? 'در حال چرخش…' : 'چرخش الان'}
          </Button>
        </div>
        <div className="cc-btnrow">
          {data.panels.map((p) => (
            <Button key={p.idx} variant={p.idx === pi ? 'primary' : 'outline'} onClick={() => setPi(p.idx)}>
              {p.name}
            </Button>
          ))}
          <Button
            variant="outline"
            onClick={async () => {
              setLogOpen(true)
              setLog(null)
              try {
                const d = await api<{ log: any[] }>(`/api/tools/hfd/log?t=${Date.now()}`)
                setLog(d.log || [])
              } catch {
                setLog([])
              }
            }}
          >
            تاریخچه
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              setRevOpen(true)
              setRevs(null)
              try {
                const d = await api<{ items: any[] }>(`/api/tools/hfd/revert?t=${Date.now()}`)
                setRevs(d.items || [])
              } catch {
                setRevs([])
              }
            }}
          >
            بازگردانی
          </Button>
        </div>
        {hErr && <div className="cc-form-err">{hErr}</div>}
        {!hosts && (
          <div className="cc-center">
            <Loader />
          </div>
        )}
        {hosts && hosts.length === 0 && <Empty title="هاستی نیست" description="این پنل هاست فعالی ندارد." />}
        {hosts?.map((h) => (
          <div key={h.id} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">
                {h.remark || h.id} {h.reality && <span className="cc-dim">· REALITY</span>}
              </div>
              <div className="cc-btnrow">
                {FIELDS.map(([f, l]) => (
                  <Button
                    key={f}
                    variant={h.fields[f] ? 'primary' : 'outline'}
                    onClick={async () => {
                      if (await act('/api/tools/hfd/field', { panel: pi, host: h.id, field: f })) loadHosts(pi)
                    }}
                  >
                    {l}
                  </Button>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>

      {logOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setLogOpen(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Title className="cc-dialog-title">تاریخچهٔ چرخش روزانه</Dialog.Title>
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <div className="cc-dialog-body">
              {!log && (
                <div className="cc-center">
                  <Loader />
                </div>
              )}
              {log && log.length === 0 && <Empty title="خالی" description="چیزی ثبت نشده." />}
              {log && log.length > 0 && (
                <div className="cc-form">
                  {log.map((e, i) => (
                    <div key={i} className="cc-sub cc-ltr">
                      {String(e.ts || '').replace('T', ' ').slice(0, 16)} · {e.kind} {e.remark ? `· ${e.remark}` : ''}{' '}
                      {e.error ? `⚠️ ${e.error}` : ''}
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="cc-dialog-actions">
              <Button variant="outline" onClick={() => setLogOpen(false)}>
                بستن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {revOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setRevOpen(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Title className="cc-dialog-title">بازگردانی چرخش روزانه</Dialog.Title>
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <div className="cc-dialog-body">
              {!revs && (
                <div className="cc-center">
                  <Loader />
                </div>
              )}
              {revs && revs.length === 0 && <Empty title="خالی" description="چیزی برای بازگردانی نیست." />}
              {revs?.map((r) => (
                <div key={r.key} className="cc-trow">
                  <div className="cc-grow">
                    <div className="cc-name cc-ltr">{r.key}</div>
                    <div className="cc-sub">{faNum(r.prev)} مقدار قبلی ذخیره شده</div>
                  </div>
                  <Button
                    variant="outline"
                    onClick={async () => {
                      if (await act('/api/tools/hfd/revert', { panel: r.panel, host: r.host }, 'بازگردانی شد')) {
                        setRevOpen(false)
                        loadHosts(pi)
                      }
                    }}
                  >
                    بازگردان
                  </Button>
                </div>
              ))}
            </div>
            <div className="cc-dialog-actions">
              {fErr && <div className="cc-form-err">{fErr}</div>}
              <Button variant="outline" onClick={() => setRevOpen(false)}>
                بستن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
    </>
  )
}

// ---------------- quick record: add + search/bulk change ----------------

const FIELDS: [string, string][] = [['address', 'آدرس'], ['sni', 'SNI'], ['host', 'Host']]

export default HfdView
