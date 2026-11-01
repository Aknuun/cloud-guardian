import { useEffect, useState } from 'react'
import { Plus, Trash } from '@phosphor-icons/react'
import { api, invalidateCache } from '../lib'
import { Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { FilterBar } from '../components/FilterBar'
import { usePageAction } from '../components/PageAction'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { useTool, } from './shared'

type MailZone = { acc: number; zid: string; zname: string; dest?: string }
type MailData = { fwd: MailZone[]; worker: MailZone[] }

function MailView() {
  usePageAction({ label: 'ایمیل دامنه جدید', icon: Plus, onClick: () => setOpen(true) })

  const { data, err, reload } = useTool<MailData>('/api/tools/mail')
  const zones = useTool<{ zones: CfZone[] }>('/api/cf/zones')
  const [open, setOpen] = useState(false)
  const [notice, setNotice] = useState('')
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')

  async function off(z: MailZone) {
    try {
      await api('/api/tools/mail/off', { acc: z.acc, zid: z.zid })
      invalidateCache()
      reload()
      setNotice('دریافت/فوروارد «' + z.zname + '» قطع شد')
    } catch (e: any) {
      setNotice(e.message || 'انجام نشد')
    }
  }
  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!data)
    return (
      <Skeleton rows={5} />
    )
  const all = [...data.fwd, ...data.worker]
  const shownFwd = data.fwd.filter((z) => flt !== 'worker' && (!q || z.zname.includes(q) || (z.dest || '').includes(q)))
  const shownWorker = data.worker.filter((z) => flt !== 'fwd' && (!q || z.zname.includes(q)))
  return (
    <>
      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی دامنه یا ایمیل…"
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'fwd', label: 'فوروارد' },
          { id: 'worker', label: 'ورکر' },
        ]}
        value={flt}
        onValue={setFlt}
      />
      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      {all.length === 0 && <Empty title="دامنه‌ای وصل نیست" description="با دکمهٔ بالا ایمیل سازمانی یک دامنه را فوروارد کن." />}

      {shownFwd.length > 0 && (
        <div className="cc-section">
          <div className="cc-section-head">
            <h2>فوروارد به ایمیل</h2>
          </div>
          {shownFwd.map((z) => (
            <div key={`${z.acc}|${z.zid}`} className="cc-trow">
              <div className="cc-grow">
                <div className="cc-name">{z.zname}</div>
                <div className="cc-sub cc-ltr">← {z.dest}</div>
              </div>
              <button className="cc-rowbtn danger" onClick={() => off(z)} title="قطع">
                <Trash size={16} />
              </button>
            </div>
          ))}
        </div>
      )}

      {shownWorker.length > 0 && (
        <div className="cc-section">
          <div className="cc-section-head">
            <h2>دریافت با ورکر</h2>
          </div>
          {shownWorker.map((z) => (
            <div key={`${z.acc}|${z.zid}`} className="cc-trow">
              <div className="cc-grow">
                <div className="cc-name">{z.zname}</div>
                <div className="cc-sub">Email Worker</div>
              </div>
              <button className="cc-rowbtn danger" onClick={() => off(z)} title="قطع">
                <Trash size={16} />
              </button>
            </div>
          ))}
        </div>
      )}

      {open && zones.data && (
        <MailDialog
          zones={zones.data.zones || []}
          onClose={() => setOpen(false)}
          onDone={(m) => {
            setOpen(false)
            setNotice(m)
            invalidateCache()
            reload()
          }}
        />
      )}
    </>
  )
}

function MailDialog({ zones, onClose, onDone }: { zones: CfZone[]; onClose: () => void; onDone: (m: string) => void }) {
  const [z, setZ] = useState('')
  const [dest, setDest] = useState('')
  const [list, setList] = useState<{ verified: any[]; pending: any[] } | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const zone = zones.find((x) => `${x.acc}|${x.id}` === z)

  useEffect(() => {
    if (!zone) return
    setList(null)
    api<{ verified: any[]; pending: any[] }>(`/api/tools/mail/zone?acc=${zone.acc}&zid=${zone.id}&t=${Date.now()}`)
      .then((d) => setList({ verified: d.verified || [], pending: d.pending || [] }))
      .catch(() => setList({ verified: [], pending: [] }))
  }, [z])

  async function submit() {
    if (!zone) return setErr('دامنه را انتخاب کن')
    if (!dest.trim()) return setErr('ایمیل مقصد لازم است')
    setBusy(true)
    setErr('')
    try {
      const known = (list?.verified || []).some((d) => String(d.email) === dest.trim())
      if (!known) {
        await api('/api/tools/mail/dest', { acc: zone.acc, zid: zone.id, email: dest.trim() })
        setBusy(false)
        onDone('مقصد اضافه شد؛ لینک تأیید به همان ایمیل رفت. بعد از تأیید دوباره اینجا انتخابش کن.')
        return
      }
      await api('/api/tools/mail/set', { acc: zone.acc, zid: zone.id, dest: dest.trim() })
      onDone('فوروارد فعال شد')
    } catch (e: any) {
      setErr(e.message || 'انجام نشد')
    }
    setBusy(false)
  }
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Title className="cc-dialog-title">ایمیل دامنه</Dialog.Title>
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <div className="cc-dialog-body">
          <div className="cc-form">
            <label className="cc-field">
              <span>دامنه</span>
              <select className="cc-input" value={z} onChange={(e) => setZ(e.target.value)}>
                <option value="">— انتخاب دامنه —</option>
                {zones.map((x) => (
                  <option key={`${x.acc}|${x.id}`} value={`${x.acc}|${x.id}`}>
                    {x.name} · {x.accName}
                  </option>
                ))}
              </select>
            </label>
            {zone && !list && (
              <div className="cc-center">
                <Loader />
              </div>
            )}
            {list && (
              <>
                <label className="cc-field">
                  <span>ایمیل مقصد</span>
                  <select className="cc-input" value={list.verified.some((d) => String(d.email) === dest) ? dest : ''} onChange={(e) => setDest(e.target.value)}>
                    <option value="">— مقصد تأییدشده —</option>
                    {list.verified.map((d) => (
                      <option key={String(d.email)} value={String(d.email)}>
                        {String(d.email)}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="cc-field">
                  <span>یا ایمیل جدید</span>
                  <Input value={dest} onChange={(e) => setDest(e.target.value)} placeholder="you@gmail.com" className="cc-ltr" />
                </label>
                {list.pending.length > 0 && (
                  <div className="cc-hint">
                    در انتظار تأیید: {list.pending.map((d) => String(d.email)).join('، ')}
                  </div>
                )}
              </>
            )}
            <div className="cc-hint">همهٔ آدرس‌های دامنه به این ایمیل فوروارد می‌شوند (catch-all).</div>
            {err && <ErrBox msg={err} />}
          </div>
        </div>
        <div className="cc-dialog-actions">
          <Button variant="outline" onClick={onClose}>
            انصراف
          </Button>
          <Button variant="primary" disabled={busy} onClick={submit}>
            {busy ? 'در حال ذخیره…' : 'ذخیره'}
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

// ---------------- settings ----------------

type CfZone = { id: string; name: string; acc: number; accName: string }

export default MailView
