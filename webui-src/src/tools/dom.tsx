import { useEffect, useState } from 'react'
import { ArrowClockwise, Plus, Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Switch } from '../components/ui'
import { StatGrid } from '../components/Stats'
import { FilterBar } from '../components/FilterBar'
import { usePageActions } from '../components/PageAction'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { SectionHeader } from '../components/SectionHeader'
import { ListRow, RowMenu, SettingsCard, SettingsRow } from '../components/ui-kit'
import { FormDialog } from './shared'
import { useTool, faTxt } from './shared'

type DomSsl = { host: string; ok: boolean; days: number; expiry: string; error: string }
type DomItem = { domain: string; status: string; expiry: number | null; manual: boolean; remaining: string; message?: string }
type DomData = { ssl: DomSsl[]; doms: DomItem[]; key: boolean; sslOn: boolean; warnDays: number }


const DOM_STATUS: Record<string, string> = {
  ok: '',
  hidden: 'تاریخ توسط ایرنیک مخفی شده',
  no_date: 'تاریخ انقضا پیدا نشد',
  not_found: 'دامنه یافت نشد',
  available: 'دامنه آزاد است',
  api_error: 'خطا در استعلام از سرویس',
  invalid: 'نام دامنه نامعتبر',
  error: 'استعلام انجام نشد',
}

type DomCfg = { provider: string; hasKey: boolean; keyHint: string; providers: { id: string; label: string }[] }

// Provider + API key for the domain-expiry lookup, with a live test.
function DomCfgDialog({ onClose }: { onClose: () => void }) {
  const [cfg, setCfg] = useState<DomCfg | null>(null)
  const [provider, setProvider] = useState('')
  const [key, setKey] = useState('')
  const [test, setTest] = useState<{ status: string; message?: string; remaining?: string; domain?: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    api<DomCfg>('/api/tools/dom/cfg').then((d) => {
      setCfg(d)
      setProvider(d.provider)
    }).catch((e) => setErr(String(e)))
  }, [])

  async function runTest() {
    setBusy(true)
    setErr('')
    try {
      const r = await api<{ status: string; message?: string; remaining?: string; domain?: string }>(
        '/api/tools/dom/cfg/test',
        { domain: '' },
      )
      setTest(r)
    } catch (e) {
      setErr(String(e))
    }
    setBusy(false)
  }

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">تنظیمات استعلام دامنه</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc">
          بدون کلید، فقط سرویس‌های آزاد (RDAP/whois) استفاده می‌شود؛ بعضی دامنه‌ها تاریخ را مخفی می‌کنند.
        </Dialog.Description>
        <div className="cc-form">
          <label className="cc-field">
            <span>روش استعلام</span>
            <select className="cc-input" value={provider} onChange={(e) => setProvider(e.target.value)}>
              {(cfg?.providers || []).map((p) => (
                <option key={p.id} value={p.id}>{p.label}</option>
              ))}
            </select>
          </label>
          <label className="cc-field">
            <span>کلید API {cfg?.hasKey ? `(ذخیره‌شده: ${cfg.keyHint})` : '(اختیاری)'}</span>
            <Input className="cc-input" value={key} onChange={(e) => setKey(e.target.value)} placeholder="برای تغییر نکردن خالی بگذار" autoComplete="off" />
          </label>
          {test && (
            <div className="cc-hint">
              {test.domain}: {test.status}
              {test.remaining ? ` — ${test.remaining}` : ''}
              {test.message ? ` (${test.message})` : ''}
            </div>
          )}
          {err && <ErrBox msg={err} />}
        </div>
        <div className="cc-dialog-actions">
          <Button variant="outline" onClick={onClose}>بستن</Button>
          <Button variant="outline" onClick={runTest} disabled={busy}>{busy ? 'در حال تست…' : 'تست'}</Button>
          <Button
            variant="primary"
            onClick={async () => {
              setBusy(true)
              try {
                await api('/api/tools/dom/cfg', { provider, key })
                invalidateCache()
                onClose()
              } catch (e) {
                setErr(String(e))
              }
              setBusy(false)
            }}
            disabled={busy}
          >
            ذخیره
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

function DomView() {
  usePageActions([
    { label: 'افزودن گواهی', icon: Plus, onClick: () => setAddSsl(true) },
    { label: 'افزودن دامنه', icon: Plus, onClick: () => setAddDom(true) },
  ])

  const { data, err, loaded, reload } = useTool<DomData>('/api/tools/dom')
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')
  const [addSsl, setAddSsl] = useState(false)
  const [host, setHost] = useState('')
  const [addDom, setAddDom] = useState(false)
  const [dom, setDom] = useState('')
  const [dateFor, setDateFor] = useState<DomItem | null>(null)
  const [when, setWhen] = useState('')
  const [fErr, setFErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [warnOn, setWarnOn] = useState<boolean | null>(null)
  const [warnDays, setWarnDays] = useState<number | null>(null)
  const [saved, setSaved] = useState(false)
  const [cfgOpen, setCfgOpen] = useState(false)

  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!loaded || !data)
    return (
      <Skeleton rows={5} />
    )

  const needle = q.trim().toLowerCase()
  const ssl0 = needle ? data.ssl.filter((s) => s.host.includes(needle)) : data.ssl
  const ssl = flt === 'near' ? ssl0.filter((s) => !s.ok || s.days <= (warnDays ?? data.warnDays)) : ssl0
  const doms = needle ? data.doms.filter((d) => d.domain.includes(needle)) : data.doms
  const showSsl = flt !== 'dom'
  const showDom = flt !== 'ssl' && flt !== 'near'
  const warn = warnDays ?? data.warnDays
  const on = warnOn ?? data.sslOn
  const bad = data.ssl.filter((s) => !s.ok || s.days <= warn).length

  const saveCfg = async () => {
    setBusy(true)
    setFErr('')
    setSaved(false)
    try {
      await api('/api/tools/dom/sslcfg', { enabled: on, warnDays: warn })
      invalidateCache()
      setWarnOn(null)
      setWarnDays(null)
      setSaved(true)
      reload()
    } catch (e: any) {
      setFErr(e.message || 'ذخیره نشد')
    } finally {
      setBusy(false)
    }
  }

  const act = async (path: string, body: any, done?: () => void) => {
    setFErr('')
    try {
      await api(path, body)
      invalidateCache()
      done?.()
      reload()
    } catch (e: any) {
      setFErr(e.message || 'انجام نشد')
    }
  }

  const refresh = async () => {
    setBusy(true)
    setFErr('')
    try {
      await api('/api/tools/dom/refresh', {})
      invalidateCache()
      reload()
    } catch (e: any) {
      setFErr(e.message || 'استعلام نشد')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <StatGrid
        items={[
          { label: 'گواهی SSL', value: data.ssl.length, note: 'تحت بررسی' },
          { label: 'نیاز به بررسی', value: bad, of: data.ssl.length },
          { label: 'دامنه‌ها', value: data.doms.length, note: 'ثبت‌شده' },
        ]}
      />
      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی دامنه"
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'ssl', label: 'گواهی‌ها' },
          { id: 'near', label: 'نزدیک انقضا' },
          { id: 'dom', label: 'انقضای دامنه' },
        ]}
        value={flt}
        onValue={setFlt}
        menu={[{ id: 'cfg', label: 'تنظیمات استعلام' }]}
        onMenu={(id) => id === 'cfg' && setCfgOpen(true)}
        onRefresh={refresh}
        busy={busy}
      />
      {fErr && <div className="cc-form-err cc-mb">{fErr}</div>}

      {showSsl && <div className="cc-section">
        <SectionHeader title="گواهی SSL" count={faNum(data.ssl.length) + ' گواهی'} />
        <SettingsCard
          title="هشدار گواهی"
          footer={
            <>
              <Button variant="outline" onClick={saveCfg} disabled={busy}>ذخیره</Button>
              {fErr ? <span className="cc-form-err">{fErr}</span> : saved ? <span className="cc-hint">ذخیره شد</span> : null}
            </>
          }
        >
          <SettingsRow label="هشدار تلگرام" hint="وقتی گواهی به روزهای هشدار برسد، روزی یک بار پیام می‌آید.">
            <Switch checked={on} onCheckedChange={setWarnOn} aria-label="هشدار تلگرام گواهی" />
          </SettingsRow>
          <SettingsRow label="روز مانده">
            <input
              className="cc-input cc-num"
              id="ssl-warn-days"
              name="sslWarnDays"
              aria-label="روزهای هشدار گواهی"
              type="number" inputMode="numeric"
              min={1}
              max={365}
              value={warn}
              onChange={(e) => setWarnDays(Number(e.target.value))}
            />
            <span className="cc-unit">روز</span>
          </SettingsRow>
        </SettingsCard>
        {ssl.length === 0 && <Empty title="گواهی‌ای نیست" description="دامنه‌ات را اضافه کن تا انقضای SSL هر ۶ ساعت بررسی شود." action={<Button variant="primary" icon={Plus} onClick={() => setAddSsl(true)}>افزودن گواهی</Button>} />}
        {ssl.map((s) => (
          <ListRow
            key={s.host}
            title={s.host}
            meta={s.ok ? 'انقضا: ' + faDate(s.expiry) : s.error}
            trailing={
              <>
                <Badge variant={s.ok && s.days > warn ? 'success' : 'error'}>
                  {s.ok ? faNum(s.days) + ' روز' : 'خطا'}
                </Badge>
                <button className="cc-rowbtn danger" aria-label="حذف گواهی" title="حذف" onClick={() => act('/api/tools/dom/ssl', { host: s.host })}>
                  <Trash size={16} />
                </button>
              </>
            }
          />
        ))}
      </div>}

      {showDom && <div className="cc-section">
        <SectionHeader
          title="انقضای دامنه"
          count={faNum(doms.length) + ' دامنه'}
          action={
            <>
              <Button variant="outline" icon={ArrowClockwise} onClick={refresh} disabled={busy}>
                استعلام دوباره
              </Button>
              <Button variant="outline" icon={Plus} onClick={() => setAddDom(true)}>
                افزودن
              </Button>
            </>
          }
        />
        {!data.key && (
          <div className="cc-hint cc-mb">
            برای دامنه‌های .ir که ایرنیک تاریخ را مخفی کرده، تاریخ را دستی ثبت کن.
          </div>
        )}
        {doms.length === 0 && <Empty title="دامنه‌ای نیست" description="تاریخ انقضا از RDAP و whois گرفته می‌شود." action={<Button variant="outline" icon={Plus} onClick={() => setAddDom(true)}>افزودن دامنه</Button>} />}
        {doms.map((d) => (
          <ListRow
            key={d.domain}
            title={d.domain}
            meta={
              (d.expiry ? 'انقضا: ' + faDate(d.expiry) : DOM_STATUS[d.status] || 'تاریخ نامشخص') +
              (!d.expiry && d.message ? ' · ' + d.message : '')
            }
            trailing={
              <>
                <Badge>{faTxt(d.remaining) || DOM_STATUS[d.status] || d.status}</Badge>
                <RowMenu
                  items={[
                    { label: 'استعلام دوباره', onClick: () => act('/api/tools/dom/one', { domain: d.domain }) },
                    {
                      label: 'ثبت تاریخ',
                      onClick: () => {
                        setFErr('')
                        setWhen('')
                        setDateFor(d)
                      },
                    },
                    { label: 'حذف', danger: true, onClick: () => act('/api/tools/dom/del', { domain: d.domain }) },
                  ]}
                />
              </>
            }
          />
        ))}
      </div>}

      {addSsl && (
        <FormDialog
          title="افزودن گواهی SSL"
          sub="روزهای باقی‌مانده گواهی هر ۶ ساعت بررسی می‌شود."
          onClose={() => setAddSsl(false)}
          onSave={() => act('/api/tools/dom/ssl', { host }, () => { setAddSsl(false); setHost('') })}
          error={fErr}
        >
          <label className="cc-field">
            <span>دامنه</span>
            <Input value={host} onChange={(e) => setHost(e.target.value)} placeholder="example.com" />
          </label>
        </FormDialog>
      )}

      {addDom && (
        <FormDialog
          title="افزودن دامنه"
          sub="انقضا از RDAP و whois.nic.ir استعلام می‌شود."
          onClose={() => setAddDom(false)}
          onSave={() => act('/api/tools/dom/add', { domain: dom }, () => { setAddDom(false); setDom('') })}
          error={fErr}
        >
          <label className="cc-field">
            <span>دامنه</span>
            <Input value={dom} onChange={(e) => setDom(e.target.value)} placeholder="example.ir" />
          </label>
        </FormDialog>
      )}

      {dateFor && (
        <FormDialog
          title="ثبت تاریخ انقضا"
          sub={dateFor.domain}
          onClose={() => setDateFor(null)}
          onSave={() => act('/api/tools/dom/date', { domain: dateFor.domain, when }, () => setDateFor(null))}
          error={fErr}
        >
          <label className="cc-field">
            <span>تاریخ شمسی</span>
            <Input value={when} onChange={(e) => setWhen(e.target.value)} placeholder="1405/06/20" />
          </label>
        </FormDialog>
      )}

      {cfgOpen && <DomCfgDialog onClose={() => setCfgOpen(false)} />}
    </>
  )
}

// ---------------- host filter (auto sub swap) ----------------

export default DomView
