import { useState } from 'react'
import { Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, invalidateCache } from '../lib'
import { Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { SectionHeader } from '../components/SectionHeader'
import { ListRow } from '../components/ui-kit'
import { useTool, verGt } from './shared'

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
  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!data)
    return (
      <Skeleton rows={5} />
    )
  const newer = !!data.latest && verGt(data.latest, data.version)
  return (
    <>
      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <SectionHeader
          title="نسخه و آپدیت"
          count={'v' + data.version}
          action={
            <Button variant="outline" disabled={busy} onClick={async () => {
              setBusy(true)
              const r = await act('/api/tools/settings/check', {}, '')
              setBusy(false)
              if (r) setNotice(r.newer ? `نسخهٔ جدید موجود است: ${r.latest}` : 'روی آخرین نسخه هستی')
              reload()
            }}>
              بررسی نسخه
            </Button>
          }
        />
        {newer && data.isMain && (
          <ListRow
            title={'آپدیت به ' + data.latest}
            meta="سرویس چند لحظه ری‌استارت می‌شود."
            trailing={
              <Button variant="outline" onClick={async () => {
                const r = await act('/api/tools/settings/update', { tag: data.latest }, '')
                if (r) setNotice('آپدیت شروع شد؛ ۱ تا ۳ دقیقه صبر کن.')
              }}>
                آپدیت
              </Button>
            }
          />
        )}
        <div className="cc-hint">
          نسخه‌ها روی {data.version ? 'همین سرور' : 'سرور'} نصب و سرویس خودکار ری‌استارت می‌شود.
        </div>
      </div>

      <div className="cc-section">
        <SectionHeader
          title="ادمین‌ها"
          count={faNum(data.admins.length) + ' ادمین'}
          action={data.isMain && (
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
        />
        {data.isMain && (
          <div className="cc-searchbar">
            <Input value={addId} onChange={(e) => setAddId(e.target.value)} placeholder="شناسهٔ عددی ادمین جدید…" />
          </div>
        )}
        {data.admins.map((id, i) => (
          <ListRow
            key={id}
            title={id}
            meta={i === 0 ? 'ادمین اصلی' : 'ادمین'}
            trailing={data.isMain && i > 0 && (
              <button className="cc-rowbtn danger" aria-label="حذف ادمین" onClick={async () => {
                const r = await act('/api/tools/settings/admin', { del: id }, 'ادمین حذف شد')
                if (r) reload()
              }}>
                <Trash size={16} />
              </button>
            )}
          />
        ))}
      </div>

      <div className="cc-section">
        <SectionHeader
          title="دستگاه‌های واردشده"
          count={faNum((sess.data?.sessions || []).length) + ' دستگاه'}
          action={
            <Button variant="outline" onClick={() => setConfirmAll(true)}>
              خروج از همه
            </Button>
          }
        />
        {sessNotice && <div className="cc-hint cc-mb">{sessNotice}</div>}
        {!sess.loaded && <Loader />}
        {(sess.data?.sessions || []).map((x) => (
          <ListRow
            key={x.id}
            title={<>{x.device || 'نامشخص'}{x.current && <span className="cc-now"> · همین دستگاه</span>}</>}
            meta={`${x.ip} · آخرین استفاده ${ago(x.last)} · ورود ${faDate(x.created)}`}
            trailing={!x.current && (
              <Button
                variant="ghost"
                aria-label="خروج دستگاه"
                onClick={async () => {
                  await act('/api/sessions/revoke', { id: x.id })
                  setSessNotice('آن دستگاه خارج شد')
                  sess.reload()
                }}
              >
                <Trash size={15} />
              </Button>
            )}
          />
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
        <SectionHeader title="راهنما" count={faNum(data.guides.length) + ' موضوع'} />
        {data.guides.map((g) => (
          <ListRow key={g.id} title={g.title} meta={g.text} onClick={() => setGuide(g)} />
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
