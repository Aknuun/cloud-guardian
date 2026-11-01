import { useEffect, useState } from 'react'
import { ArrowClockwise, BellRinging, BellSimple, Copy, PencilSimple, Plus, Power, ShieldCheck, Trash, WarningCircle } from '@phosphor-icons/react'
import { api, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { FilterBar } from '../components/FilterBar'
import { SectionHeader } from '../components/SectionHeader'
import { usePageActions } from '../components/PageAction'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, save, } from './shared'
import type { PgPanel, PgMon, PgData } from './shared'

function PgView() {
  usePageActions([
    { label: 'افزودن پنل', icon: Plus, onClick: () => setPf({ name: '', url: '', user: '', pass: '' }) },
    { label: 'افزودن مانیتور', icon: ShieldCheck, onClick: () => setMf({ panel_url: (data?.panels[0]?.url) || '', name: '', interval: 5 }) },
  ])

  const { data, err, loaded, reload } = useTool<PgData>('/api/tools/pg')
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')
  const [pf, setPf] = useState<{ idx?: number; name: string; url: string; user: string; pass: string } | null>(null)
  const [pfErr, setPfErr] = useState('')
  const [pdel, setPdel] = useState<PgPanel | null>(null)
  const [mf, setMf] = useState<{ panel_url: string; name: string; interval: number } | null>(null)
  const [mfErr, setMfErr] = useState('')
  const [mdel, setMdel] = useState<PgMon | null>(null)
  const [nd, setNd] = useState<PgMon | null>(null)
  const [node, setNode] = useState<PgPanel | null>(null)
  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!loaded) return <Skeleton />
  const iso0 = data || { panels: [], monitors: [] }
  const needle = q.trim().toLowerCase()
  const iso = needle
    ? {
        panels: iso0.panels.filter((p) => (p.name + ' ' + p.url + ' ' + p.user).toLowerCase().includes(needle)),
        monitors: iso0.monitors.filter((m) => (m.name + ' ' + m.panel).toLowerCase().includes(needle)),
      }
    : iso0
  const _on = iso.monitors.filter((m) => m.enabled).length

  const submitPanel = async () => {
    if (!pf) return
    setPfErr('')
    try {
      await save(pf.idx === undefined ? '/api/tools/pg/panel/add' : '/api/tools/pg/panel/edit', pf, () => {
        setPf(null)
        reload()
      })
    } catch (e: any) {
      setPfErr(e.message || 'ذخیره نشد')
    }
  }
  const submitMon = async () => {
    if (!mf) return
    setMfErr('')
    try {
      await save('/api/tools/pg/mon/add', mf, () => {
        setMf(null)
        reload()
      })
    } catch (e: any) {
      setMfErr(e.message || 'ذخیره نشد')
    }
  }
  const run = async (path: string, body: any) => {
    try {
      await save(path, body, reload)
    } catch (e: any) {
      alert(e.message || 'انجام نشد')
    }
  }

  return (
    <>
      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی پاسارگارد"
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'panels', label: 'پنل‌ها' },
          { id: 'mons', label: 'مانیتور نود' },
        ]}
        value={flt}
        onValue={setFlt}
        onRefresh={() => {
          invalidateCache()
          reload()
        }}
      />
      {flt !== 'mons' && (
        <SectionHeader title="پنل‌ها" count={faNum(iso.panels.length) + ' پنل متصل'} />
      )}
      {flt !== 'mons' && iso.panels.length === 0 && (
        <Empty
          title="پنلی ثبت نشده"
          description="برای مانیتور نودها و مدیریت کاربران، اول یک پنل پاسارگارد وصل کن."
          action={
            <Button variant="primary" icon={Plus} onClick={() => setPf({ name: '', url: '', user: '', pass: '' })}>
              افزودن پنل
            </Button>
          }
        />
      )}
      {flt !== 'mons' && iso.panels.length > 0 && (
        <div className="cc-panelgrid">
          {iso.panels.map((p) => (
            <div key={p.url} className="cc-pgcard">
              <div className="cc-pgcard-top">
                <span className="cc-pgcard-ico"><ShieldCheck size={19} /></span>
                <div className="cc-grow">
                  <div className="cc-name">{p.name || p.url}</div>
                  <div className="cc-sub cc-mono cc-ltr">{p.url.replace(/^https?:\/\//, '')}</div>
                </div>
                {p.user && <Badge>{p.user}</Badge>}
              </div>
              <div className="cc-pgcard-actions">
                <Button variant="primary" icon={Plus} onClick={() => setNode(p)}>
                  نود جدید
                </Button>
                <button
                  type="button"
                  aria-label="ویرایش پنل"
                  className="cc-rowbtn"
                  onClick={() => setPf({ idx: p.idx, name: p.name, url: p.url, user: p.user, pass: '' })}
                >
                  <PencilSimple size={16} />
                </button>
                <button type="button" aria-label="حذف پنل" className="cc-rowbtn danger" onClick={() => setPdel(p)}>
                  <Trash size={16} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {flt !== 'panels' && (
        <SectionHeader title="مانیتور نودها" count={faNum(iso.monitors.length) + ' مانیتور'} className="cc-mt" />
      )}
      {flt !== 'panels' && iso.monitors.length === 0 && (
        <Empty title="مانیتوری نیست" description="با دکمهٔ + پایین صفحه یک مانیتور نود بساز." />
      )}
      {flt !== 'panels' && iso.monitors.map((m) => (
        <div key={m.id} className={'cc-trow cc-tap' + (m.enabled ? '' : ' cc-off')} onClick={() => setNd(m)}>
          <span className={'cc-st' + (m.enabled ? ' up' : '')} />
          <div className="cc-grow">
            <div className="cc-name">{m.name || m.id}</div>
            <div className="cc-sub cc-ltr">{m.panel.replace(/^https?:\/\//, '')}</div>
          </div>
          <span className="cc-chip cc-chip-sm">{faNum(m.interval)} دقیقه</span>
          <div className="cc-rowbtns" onClick={(e) => e.stopPropagation()}>
            <button
              type="button"
              aria-label={m.enabled ? 'خاموش کن' : 'روشن کن'}
              onClick={() => run('/api/tools/pg/mon/toggle', { id: m.id, enabled: !m.enabled })}
            >
              <Power size={17} />
            </button>
            <button type="button" aria-label="حذف" className="danger" onClick={() => setMdel(m)}>
              <Trash size={17} />
            </button>
          </div>
        </div>
      ))}

      {node && <NodeDialog panel={node} onClose={() => setNode(null)} />}

      {nd && <NdDialog m={nd} onClose={() => setNd(null)} onChanged={reload} />}

      {pf && (
        <FormDialog
          title={pf.idx === undefined ? 'افزودن پنل' : 'ویرایش پنل'}
          sub={pf.idx === undefined ? 'ورود قبل از ذخیره تست می‌شود.' : 'رمز را خالی بگذار تا تغییر نکند.'}
          onClose={() => setPf(null)}
          onSave={submitPanel}
          error={pfErr}
        >
          <label className="cc-field">
            <span>نام</span>
            <Input value={pf.name} onChange={(e) => setPf({ ...pf, name: e.target.value })} placeholder="main" />
          </label>
          <label className="cc-field">
            <span>آدرس پنل</span>
            <Input value={pf.url} onChange={(e) => setPf({ ...pf, url: e.target.value })} placeholder="https://panel.example.com" />
          </label>
          <label className="cc-field">
            <span>نام کاربری</span>
            <Input value={pf.user} onChange={(e) => setPf({ ...pf, user: e.target.value })} placeholder="admin" />
          </label>
          <label className="cc-field">
            <span>رمز</span>
            <Input type="password" value={pf.pass} onChange={(e) => setPf({ ...pf, pass: e.target.value })} placeholder="••••••" />
          </label>
        </FormDialog>
      )}

      {mf && (
        <FormDialog title="افزودن مانیتور نود" onClose={() => setMf(null)} onSave={submitMon} error={mfErr}>
          <label className="cc-field">
            <span>پنل</span>
            <select className="cc-input" value={mf.panel_url} onChange={(e) => setMf({ ...mf, panel_url: e.target.value })}>
              {iso0.panels.map((p) => (
                <option key={p.url} value={p.url}>
                  {p.name || p.url}
                </option>
              ))}
            </select>
          </label>
          <label className="cc-field">
            <span>نام نمایشی</span>
            <Input value={mf.name} onChange={(e) => setMf({ ...mf, name: e.target.value })} placeholder="node-1" />
          </label>
          <label className="cc-field">
            <span>فاصلهٔ بررسی (دقیقه)</span>
            <Input
              type="number"
              value={String(mf.interval)}
              onChange={(e) => setMf({ ...mf, interval: Number(e.target.value) || 5 })}
            />
          </label>
        </FormDialog>
      )}

      {pdel && (
        <Dialog.Root open onOpenChange={(o) => !o && setPdel(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف پنل</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              «{pdel.name || pdel.url}» و مانیتورهای نودش حذف می‌شوند. روی پاسارگارد اثری ندارد.
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setPdel(null)}>
                انصراف
              </Button>
              <Button variant="destructive" onClick={() => pdel && run('/api/tools/pg/panel/del', { idx: pdel.idx })}>
                حذف کن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {mdel && (
        <Dialog.Root open onOpenChange={(o) => !o && setMdel(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف مانیتور</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">مانیتور «{mdel.name || mdel.id}» حذف شود؟</Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setMdel(null)}>
                انصراف
              </Button>
              <Button variant="destructive" onClick={() => mdel && run('/api/tools/pg/mon/del', { id: mdel.id })}>
                حذف کن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
    </>
  )
}


// ---------------- node monitor detail ----------------
type NdNode = { name: string; dir: string; addr?: string; version?: string; reason?: string; ts?: string; exc: boolean }
type NdData = { id: string; name: string; panel: string; interval: number; enabled: boolean; hook: string; nodes: NdNode[] }

function NdDialog({ m, onClose, onChanged }: { m: PgMon; onClose: () => void; onChanged: () => void }) {
  const [st, setSt] = useState<NdData | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState('')
  const [interval, setInterval] = useState(m.interval || 10)
  const [notice, setNotice] = useState('')

  const load = async () => {
    setErr('')
    try {
      setSt(await api<NdData>('/api/tools/pg/mon?id=' + encodeURIComponent(m.id) + '&t=' + Date.now()))
    } catch (e: any) {
      setErr(e.message || 'خوانده نشد')
    }
  }
  useEffect(() => {
    load()
  }, [m.id])

  const post = async (path: string, body: any, label: string) => {
    setBusy(label)
    setErr('')
    setNotice('')
    try {
      const res = await api<NdData>(path, body)
      if (res?.nodes) setSt(res)
      invalidateCache()
      onChanged()
    } catch (e: any) {
      setErr(e.message || 'انجام نشد')
    } finally {
      setBusy('')
    }
  }

  const nodes = st?.nodes || []
  const up = nodes.filter((n) => n.dir === 'up').length
  const down = nodes.filter((n) => n.dir === 'down').length

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">{m.name || m.id}</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc cc-mono">{m.panel}</Dialog.Description>

        {err && <div className="cc-form-err cc-mb">{err}</div>}
        {notice && <div className="cc-hint cc-mb">{notice}</div>}
        {!st && !err && <Loader size="base" />}

        {st && (
          <div className="cc-moncc">
            <div className="cc-mon">
              <div>
                <span>نودها</span>
                <b>{faNum(nodes.length)}</b>
              </div>
              <div>
                <span>سالم</span>
                <b>{faNum(up)}</b>
              </div>
              <div>
                <span>قطع</span>
                <b>{faNum(down)}</b>
              </div>
            </div>

            <div className="cc-field-row">
              <label className="cc-field cc-field-sm">
                <span>فاصله (دقیقه)</span>
                <Input
                  type="number"
                  value={String(interval)}
                  onChange={(e) => setInterval(Number(e.target.value) || 1)}
                />
              </label>
              <Button
                variant="outline"
                disabled={!!busy}
                onClick={() => post('/api/tools/pg/mon/cfg', { id: m.id, interval, enabled: st.enabled }, 'cfg')}
              >
                ذخیرهٔ فاصله
              </Button>
            </div>

            <div className="cc-actions">
              <Button variant="outline" icon={ArrowClockwise} disabled={!!busy} onClick={() => post('/api/tools/pg/mon/sync', { id: m.id }, 'sync')}>
                همگام‌سازی
              </Button>
              <Button variant="outline" icon={BellRinging} disabled={!!busy} onClick={() => post('/api/tools/pg/mon/test', { id: m.id }, 'test').then(() => setNotice('پیام تستی در تلگرام ارسال شد.'))}>
                تست هشدار
              </Button>
              <Button variant="outline" icon={Power} disabled={!!busy} onClick={() => post('/api/tools/pg/mon/cfg', { id: m.id, interval, enabled: !st.enabled }, 'tg')}>
                {st.enabled ? 'توقف پایش' : 'فعال‌سازی'}
              </Button>
              <Button
                variant="outline"
                icon={Copy}
                onClick={() => {
                  fallbackCopy(st.hook)
                  setNotice('آدرس وب‌هوک کپی شد.')
                }}
              >
                کپی وب‌هوک
              </Button>
            </div>
            <div className="cc-hookurl">{st.hook}</div>

            {nodes.length === 0 && <Empty title="نودی ثبت نشده" description="«همگام‌سازی» را بزن تا نودها از پنل خوانده شوند." />}
            {nodes.map((n) => (
              <div key={n.name} className="cc-trow">
                <div className="cc-grow">
                  <div className="cc-name">{n.name}</div>
                  <div className="cc-sub">
                    {[n.addr, n.version, n.reason].filter(Boolean).join(' · ') || '—'}
                  </div>
                </div>
                <Badge variant={n.dir === 'up' ? 'success' : n.dir === 'down' ? 'error' : 'neutral'}>
                  {n.dir === 'up' ? 'سالم' : n.dir === 'down' ? 'قطع' : 'نامشخص'}
                </Badge>
                <button
                  type="button"
                  className={n.exc ? 'cc-rowbtn' : 'cc-rowbtn danger'}
                  title={n.exc ? 'حذف از استثنا' : 'استثنا کردن این نود'}
                  disabled={!!busy}
                  onClick={() => post('/api/tools/pg/mon/exc', { id: m.id, node: n.name }, 'exc')}
                >
                  <BellSimple size={16} />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="cc-dialog-actions">
          <Button variant="secondary" onClick={onClose}>
            بستن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

/* ---------------- server extras ---------------- */

export default PgView

function NodeDialog({ panel, onClose }: { panel: PgPanel; onClose: () => void }) {
  const [data, setData] = useState<{ script: string; url: string; user: string; panel: string } | null>(null)
  const [err, setErr] = useState('')
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    api<{ script: string; url: string; user: string; panel: string }>('/api/tools/pg/node', { idx: panel.idx })
      .then(setData)
      .catch((e) => setErr(e.message || 'ساخت اسکریپت ناموفق بود'))
  }, [panel.idx])
  const copy = async () => {
    if (!data) return
    await fallbackCopy(data.script)
    setCopied(true)
    setTimeout(() => setCopied(false), 1800)
  }
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">افزودن نود به «{panel.name || panel.url}»</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc">
          اسکریپت زیر مخصوص <b>همین پنل</b> ساخته شده و روی سرور نودی که می‌خواهی عضو این پنل شود اجرا می‌شود.
        </Dialog.Description>

        <ol className="cc-steps-list">
          <li>با SSH به سرور نود وصل شو (Ubuntu/Debian تازه).</li>
          <li>دستور زیر را کامل کپی و اجرا کن؛ نصب و اتصال خودکار انجام می‌شود.</li>
          <li>حدود یک دقیقه بعد، نود در همین پنل و در فهرست مانیتورها ظاهر می‌شود.</li>
        </ol>

        <div className="cc-warnbox">
          <WarningCircle size={18} />
          <span>
            رمز پنل داخل این دستور است. آن را فقط روی سرور خودت اجرا کن و در چت، گروه یا جای عمومی نفرست.
            اگر لو رفت، رمز پنل را عوض کن.
          </span>
        </div>

        {err && <div className="cc-form-err">{err}</div>}
        {!data && !err && <Loader size="sm" />}
        {data && (
          <>
            <div className="cc-codewrap">
              <button type="button" className="cc-codecopy" onClick={copy} aria-label="کپی اسکریپت">
                <Copy size={15} /> {copied ? 'کپی شد' : 'کپی'}
              </button>
              <pre className="cc-code cc-ltr">{data.script}</pre>
            </div>
            <div className="cc-hint">
              پنل: <span className="cc-ltr cc-mono">{data.url}</span> · کاربر: <span className="cc-ltr cc-mono">{data.user}</span>
            </div>
          </>
        )}

        <div className="cc-dialog-actions">
          <Button variant="secondary" onClick={onClose}>
            بستن
          </Button>
          <Button variant="primary" icon={Copy} onClick={copy} disabled={!data}>
            {copied ? 'کپی شد' : 'کپی اسکریپت'}
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}
