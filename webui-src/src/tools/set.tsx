import { useEffect, useState } from 'react'
import { CaretLeft, Lightning, Envelope, GearSix, Lifebuoy, Broadcast, ArrowsClockwise, CalendarDots, BellSimple, Fire, GlobeSimple, Pulse, ArrowClockwise, BellRinging, Check, Copy, Gear, HardDrive, Info, PencilSimple, Plus, Power, ShieldCheck, Terminal, Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { StatGrid } from '../components/Stats'
import { FilterBar } from '../components/FilterBar'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, save, faTxt, verGt } from './shared'
import type { Srv, Rem, PgPanel, PgMon, PgData } from './shared'

type Sess = { id: string; created: number; last: number; exp: number; ip: string; device: string; current: boolean }

type SetData = { version: string; latest: string; isMain: boolean; admins: number[]; guides: { id: string; title: string; text: string }[] }

function SetView() {
  const { data, err, reload } = useTool<SetData>('/api/tools/settings')
  const sess = useTool<{ sessions: Sess[] }>('/api/sessions')
  const [sessNotice, setSessNotice] = useState('')
  const [confirmAll, setConfirmAll] = useState(false)
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [addId, setAddId] = useState('')
  const [guide, setGuide] = useState<{ title: string; text: string } | null>(null)

  async function act(path: string, body: any, msg?: string) {
    setNotice('')
    try {
      const r = await api<any>(path, body)
      invalidateCache()
      if (msg) setNotice(msg)
      return r
    } catch (e: any) {
      setNotice(e.message || 'انجام نشد')
      return null
    }
  }
  if (err) return <Empty title="خطا" description={err} />
  if (!data)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
    )
  const newer = !!data.latest && verGt(data.latest, data.version)
  return (
    <>
      <StatGrid
        items={[
          { label: 'نسخهٔ نصب‌شده', value: 'v' + data.version },
          { label: 'آخرین نسخه', value: data.latest || '—' },
          { label: 'ادمین‌ها', value: faNum(data.admins.length) },
        ]}
      />
      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>نسخه و آپدیت</h2>
          <Button variant="outline" disabled={busy} onClick={async () => {
            setBusy(true)
            const r = await act('/api/tools/settings/check', {}, '')
            setBusy(false)
            if (r) setNotice(r.newer ? `نسخهٔ جدید موجود است: ${r.latest}` : 'روی آخرین نسخه هستی')
            reload()
          }}>
            بررسی نسخه
          </Button>
        </div>
        {newer && data.isMain && (
          <div className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">آپدیت به {data.latest}</div>
              <div className="cc-sub">سرویس چند لحظه ری‌استارت می‌شود.</div>
            </div>
            <Button variant="primary" onClick={async () => {
              const r = await act('/api/tools/settings/update', { tag: data.latest }, '')
              if (r) setNotice('آپدیت شروع شد؛ ۱ تا ۳ دقیقه صبر کن.')
            }}>
              آپدیت
            </Button>
          </div>
        )}
        <div className="cc-hint">
          نسخه‌ها روی {data.version ? 'همین سرور' : 'سرور'} نصب و سرویس خودکار ری‌استارت می‌شود.
        </div>
      </div>

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>ادمین‌ها</h2>
          {data.isMain && (
            <Button variant="outline" onClick={async () => {
              if (!/^\d+$/.test(addId.trim())) return setNotice('شناسهٔ عددی بفرست')
              const r = await act('/api/tools/settings/admin', { add: Number(addId) }, 'ادمین اضافه شد')
              if (r) {
                setAddId('')
                reload()
              }
            }}>
              افزودن ادمین
            </Button>
          )}
        </div>
        {data.isMain && (
          <div className="cc-searchbar">
            <Input value={addId} onChange={(e) => setAddId(e.target.value)} placeholder="شناسهٔ عددی ادمین جدید…" />
          </div>
        )}
        {data.admins.map((id, i) => (
          <div key={id} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name cc-ltr">{id}</div>
              <div className="cc-sub">{i === 0 ? 'ادمین اصلی' : 'ادمین'}</div>
            </div>
            {data.isMain && i > 0 && (
              <button className="cc-rowbtn danger" onClick={async () => {
                const r = await act('/api/tools/settings/admin', { del: id }, 'ادمین حذف شد')
                if (r) reload()
              }}>
                <Trash size={16} />
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>دستگاه‌های واردشده</h2>
          <Button variant="outline" onClick={() => setConfirmAll(true)}>
            خروج از همه
          </Button>
        </div>
        {sessNotice && <div className="cc-hint cc-mb">{sessNotice}</div>}
        {!sess.loaded && <Loader />}
        {(sess.data?.sessions || []).map((x) => (
          <div key={x.id} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">
                {x.device || 'نامشخص'}
                {x.current && <span className="cc-now"> · همین دستگاه</span>}
              </div>
              <div className="cc-sub cc-ltr">
                {x.ip} · آخرین استفاده {ago(x.last)} · ورود {faDate(x.created)}
              </div>
            </div>
            {!x.current && (
              <Button
                variant="ghost"
                onClick={async () => {
                  await act('/api/sessions/revoke', { id: x.id })
                  setSessNotice('آن دستگاه خارج شد')
                  sess.reload()
                }}
              >
                <Trash size={15} />
              </Button>
            )}
          </div>
        ))}
        {sess.loaded && (sess.data?.sessions || []).length === 0 && <Empty title="نشستی نیست" />}
      </div>

      {confirmAll && (
        <Dialog.Root open onOpenChange={(o) => !o && setConfirmAll(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">خروج از همهٔ دستگاه‌ها</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              همهٔ نشست‌ها باطل می‌شوند و روی هر دستگاه باید دوباره از تلگرام وارد شوی — از جمله همین دستگاه.
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setConfirmAll(false)}>
                انصراف
              </Button>
              <Button
                variant="primary"
                onClick={async () => {
                  await act('/api/sessions/revoke', { all: true })
                  setConfirmAll(false)
                  window.location.href = '/app/'
                }}
              >
                خارج کن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>راهنما</h2>
        </div>
        {data.guides.map((g) => (
          <div key={g.id} className="cc-trow cc-tap" onClick={() => setGuide(g)}>
            <div className="cc-grow">
              <div className="cc-name">{g.title}</div>
              <div className="cc-sub">{g.text}</div>
            </div>
          </div>
        ))}
      </div>

      {guide && (
        <Dialog.Root open onOpenChange={(o) => !o && setGuide(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Title className="cc-dialog-title">{guide.title}</Dialog.Title>
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <div className="cc-dialog-body">
              <div className="cc-hint">{guide.text}</div>
            </div>
            <div className="cc-dialog-actions">
              <Button variant="outline" onClick={() => setGuide(null)}>
                بستن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
    </>
  )
}

function ago(ts: number) {
  const m = Math.round((Date.now() / 1000 - ts) / 60)
  if (m < 1) return 'همین حالا'
  if (m < 60) return faNum(m) + ' دقیقه پیش'
  const h = Math.round(m / 60)
  if (h < 24) return faNum(h) + ' ساعت پیش'
  return faNum(Math.round(h / 24)) + ' روز پیش'
}

export default SetView
