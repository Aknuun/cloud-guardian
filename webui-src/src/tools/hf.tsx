import { useEffect, useState } from 'react'
import { Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, invalidateCache } from '../lib'
import { Button, Dialog, Empty, Input, Loader, Switch } from '../components/ui'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { SectionHeader } from '../components/SectionHeader'
import { RunStrip } from '../components/RunStrip'
import { ChipGroup, ListRow, SettingsCard, SettingsRow } from '../components/ui-kit'
import { useTool, } from './shared'

type HfCfg = {
  enabled: boolean; intervalMin: number; cities: number; citiesSel: string[]; maxOk: number
  recheckCount: number; recheckMin: number; batch: number; maxChanges: number
  realityRotate: boolean; lastRun: string; lastSummary: string; excCount: number
}
type HfData = { cfg: HfCfg; panels: { idx: number; name: string; url: string }[] }
type HfHost = {
  id: string; remark: string; disabled: boolean; reality: boolean
  address: string[]; sni: string[]; host: string[]; skip: Record<string, boolean>; all: boolean
}

const FIELDS: [string, string][] = [['address', 'آدرس'], ['sni', 'SNI'], ['host', 'Host']]

function HfView() {
  const { data, err, reload } = useTool<HfData>('/api/tools/hf')
  const [cfg, setCfg] = useState<HfCfg | null>(null)
  const [pi, setPi] = useState(0)
  const [hosts, setHosts] = useState<HfHost[] | null>(null)
  const [hErr, setHErr] = useState('')
  const [notice, setNotice] = useState('')
  const [fErr, setFErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  const [logOpen, setLogOpen] = useState(false)
  const [log, setLog] = useState<any[] | null>(null)
  const [revOpen, setRevOpen] = useState(false)
  const [revs, setRevs] = useState<{ sid: string; from?: string; to?: string }[] | null>(null)
  const [bkOpen, setBkOpen] = useState(false)

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
      const d = await api<{ hosts: HfHost[] }>(`/api/tools/hf/hosts?panel=${p}&t=${Date.now()}`)
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
  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!data || !cfg)
    return (
      <Skeleton rows={5} />
    )
  const num = (k: keyof HfCfg, label: string, unit: string) => (
    <SettingsRow label={label}>
      <Input className="cc-num" type="number" value={String(cfg[k])} onChange={(e) => setCfg({ ...cfg, [k]: Number(e.target.value) || 0 } as HfCfg)} />
      <span className="cc-unit">{unit}</span>
    </SettingsRow>
  )
  const submit = async () => {
    setSaved(false)
    if (await act('/api/tools/hf/cfg', cfg)) {
      setSaved(true)
      reload()
    }
  }
  return (
    <>
      <div className="cc-hint cc-mb">
        دامنهٔ هاست‌ها هر {faNum(cfg.intervalMin)} دقیقه از داخل ایران بررسی می‌شود؛ اگر فیلتر شد، ساب‌دامنهٔ تازه ساخته و
        جایگزین می‌شود. هاست‌هایی که لازم نیست بررسی شوند را در لیست پایین «استثنا» کن.
      </div>
      <RunStrip ts={cfg.lastRun} text={cfg.lastSummary} />

      <SettingsCard
        title="تنظیمات تعویض"
        footer={
          <>
            <Button variant="primary" onClick={submit}>ذخیرهٔ تنظیمات</Button>
            {fErr ? <span className="cc-form-err">{fErr}</span> : saved ? <span className="cc-hint">ذخیره شد</span> : null}
          </>
        }
      >
        <SettingsRow label="تعویض خودکار">
          <Switch checked={!!cfg.enabled} onCheckedChange={(v) => setCfg({ ...cfg, enabled: v })} aria-label="تعویض خودکار فعال باشد" />
        </SettingsRow>
        {num('intervalMin', 'فاصلهٔ اجرا', 'دقیقه')}
        {num('cities', 'حداقل شهر فیلتر', 'شهر')}
        {num('maxOk', 'حداکثر پینگ موفق', 'از ۴')}
        {num('batch', 'تعداد بررسی هر اجرا', 'هاست')}
        {num('maxChanges', 'حداکثر تعویض', 'هاست')}
        {num('recheckCount', 'تعداد بررسی مجدد', 'بار')}
        {num('recheckMin', 'فاصلهٔ بررسی مجدد', 'دقیقه')}
      </SettingsCard>

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <SectionHeader
          title="هاست‌ها"
          count={faNum(hosts?.length || 0) + ' هاست'}
          action={
            <Button
              variant="outline"
              disabled={busy}
              onClick={async () => {
                setBusy(true)
                const ok = await act('/api/tools/hf/run', {}, 'بررسی شروع شد؛ نتیجه در تلگرام می‌آید')
                setBusy(false)
                if (ok) setNotice('بررسی شروع شد؛ نتیجه در تلگرام می‌آید')
              }}
            >
              {busy ? 'در حال اجرا…' : 'بررسی الان'}
            </Button>
          }
        />
        <ChipGroup
          items={data.panels.map((p) => ({ id: String(p.idx), label: p.name }))}
          value={String(pi)}
          onChange={(id) => setPi(Number(id))}
        />
        <div className="cc-actions">
          <Button variant="outline" onClick={() => setBkOpen(true)}>
            بکاپ‌ها
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              setLogOpen(true)
              setLog(null)
              try {
                const d = await api<{ log: any[] }>(`/api/tools/hf/log?t=${Date.now()}`)
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
              if (await act('/api/tools/hf/exc', { panel: pi, clear: true })) loadHosts(pi)
            }}
          >
            پاک‌کردن استثناها
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              for (const h of hosts || []) {
                if (h.all) continue
                await act('/api/tools/hf/exc', { panel: pi, host: h.id, all: true })
              }
              loadHosts(pi)
            }}
          >
            استثنا کردن همه
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              setRevOpen(true)
              setRevs(null)
              try {
                const d = await api<{ items: any[] }>(`/api/tools/hf/revert?t=${Date.now()}`)
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
              <div className="cc-sub">{(h.address || []).join('، ') || '—'}</div>
              {(h.sni || []).length > 0 && <div className="cc-sub">sni: {(h.sni || []).join('، ')}</div>}
              {(h.host || []).length > 0 && <div className="cc-sub">host: {(h.host || []).join('، ')}</div>}
              <ChipGroup
                items={FIELDS.map(([id, label]) => ({ id, label }))}
                value={FIELDS.filter(([f]) => !h.skip[f]).map(([f]) => f)}
                onChange={async (f) => {
                  if (await act('/api/tools/hf/exc', { panel: pi, host: h.id, field: f })) loadHosts(pi)
                }}
              />
            </div>
          </div>
        ))}
      </div>

      {bkOpen && (
        <BkDialog
          onClose={() => setBkOpen(false)}
          act={act}
          faTxt={(v) => v}
        />
      )}

      {logOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setLogOpen(false)}>
          <Dialog className="cc-dialog" size="base">
              <Dialog.Title className="cc-dialog-title">تاریخچهٔ تعویض‌ها</Dialog.Title>
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
                    {[...log].reverse().map((e, i) => (
                      <div key={i} className="cc-sub cc-ltr">
                        {String(e.ts || '').replace('T', ' ').slice(0, 16)} · {e.kind} {e.from ? `${e.from} → ${e.to}` : ''}{' '}
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
              <Dialog.Title className="cc-dialog-title">بازگردانی تعویض‌ها</Dialog.Title>
              <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
              <div className="cc-dialog-body">
                {!revs && (
                  <div className="cc-center">
                    <Loader />
                  </div>
                )}
                {revs && revs.length === 0 && <Empty title="خالی" description="تعویضی برای بازگردانی نیست." />}
                {revs?.map((r) => (
                  <ListRow
                    key={r.sid}
                    title={r.from || r.sid}
                    meta={r.to ? '→ ' + r.to : undefined}
                    trailing={
                      <Button
                        variant="outline"
                        onClick={async () => {
                          if (await act('/api/tools/hf/revert', { sid: r.sid }, 'بازگردانی شد')) {
                            setRevOpen(false)
                            loadHosts(pi)
                          }
                        }}
                      >
                        بازگردان
                      </Button>
                    }
                  />
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

// ---------------- pasarguard expiry / volume webhook ----------------
function BkDialog({ onClose, act }: { onClose: () => void; act: (p: string, b: any, m?: string) => Promise<boolean>; faTxt: (v: string) => string }) {
  const [items, setItems] = useState<any[] | null>(null)
  const [busy, setBusy] = useState(false)
  const load = async () => {
    try {
      const d = await api<{ items: any[] }>(`/api/tools/hf/bk?t=${Date.now()}`)
      setItems(d.items || [])
    } catch {
      setItems([])
    }
  }
  useEffect(() => {
    load()
  }, [])
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Title className="cc-dialog-title">بکاپ هاست‌ها</Dialog.Title>
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <div className="cc-dialog-body">
          <div className="cc-actions">
            <Button
              variant="primary"
              disabled={busy}
              onClick={async () => {
                setBusy(true)
                await act('/api/tools/hf/bk', { op: 'new' }, 'بکاپ گرفته شد')
                setBusy(false)
                load()
              }}
            >
              گرفتن بکاپ جدید
            </Button>
          </div>
          {!items && (
            <div className="cc-center">
              <Loader />
            </div>
          )}
          {items && items.length === 0 && <Empty title="بکاپی نیست" description="اولین بکاپ را بگیر." />}
          {items?.map((b) => (
            <ListRow
              key={b.id}
              title={faDate(b.ts)}
              meta={b.label + ' · ' + faNum(b.count) + ' هاست'}
              trailing={
                <>
                  <Button
                    variant="outline"
                    onClick={async () => {
                      if (await act('/api/tools/hf/bk', { op: 'restore', id: b.id }, 'بازگردانی انجام شد')) onClose()
                    }}
                  >
                    بازگردان
                  </Button>
                  <button
                    className="cc-rowbtn danger"
                    aria-label="حذف بکاپ"
                    onClick={async () => {
                      await act('/api/tools/hf/bk', { op: 'del', id: b.id }, 'حذف شد')
                      load()
                    }}
                  >
                    <Trash size={16} />
                  </button>
                </>
              }
            />
          ))}
        </div>
        <div className="cc-dialog-actions">
          <Button variant="outline" onClick={onClose}>
            بستن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

// verGt reports whether version a is newer than b (both "x.y.z" or "vx.y.z").
function _verGt(a: string, b: string) {
  const pa = String(a).replace(/^v/, '').split('.').map((x) => Number(x) || 0)
  const pb = String(b).replace(/^v/, '').split('.').map((x) => Number(x) || 0)
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] || 0) - (pb[i] || 0)
    if (d) return d > 0
  }
  return false
}

export default HfView
