import { useState } from 'react'
import { ArrowClockwise, Copy, Plus, Power, Trash } from '@phosphor-icons/react'
import { faNum, fallbackCopy, } from '../lib'
import { Badge, Button, Dialog, Empty, Input, } from '../components/ui'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, save, } from './shared'

type FoData = {
  configured: boolean
  enabled: boolean
  worker: string
  url: string
  account: string
  namespace: string
  health: string
  publicUrl: string
  owner: 'server' | 'worker' | 'other'
  serverUp: boolean
  probeCode: number
  webhook: string
}

function FoView() {
  const { data, err, loaded, reload } = useTool<FoData>('/api/tools/fo')
  const [form, setForm] = useState<{ token: string; name: string } | null>(null)
  const [fErr, setFErr] = useState('')
  const [busy, setBusy] = useState('')
  const [notice, setNotice] = useState('')
  const [del, setDel] = useState(false)

  const act = async (path: string, body: any, msg: string) => {
    setBusy(path)
    setNotice('')
    try {
      await save(path, body, () => setNotice(msg))
      reload()
    } catch (e: any) {
      setNotice(String(e?.message || e))
    } finally {
      setBusy('')
    }
  }

  const d = data
  const _ownerTxt = d?.owner === 'worker' ? 'ورکر پشتیبان' : d?.owner === 'other' ? 'نامشخص' : 'سرور'

  return (
    <>

      <div className="cc-pagebar">
        <Button variant="primary" icon={Plus} onClick={() => { setFErr(''); setForm({ token: '', name: '' }) }}>
          {d?.configured ? 'تنظیم مجدد / تغییر ورکر' : 'راه‌اندازی فیلور'}
        </Button>
      </div>

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}
      {err && <ErrBox msg={err} onRetry={reload} />}
      {!loaded && <Skeleton rows={3} />}

      {d?.configured && (
        <div className="cc-section">
          <div className="cc-section-head"><h2>ورکر پشتیبان</h2></div>
          <div className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">{d.worker}</div>
              <div className="cc-sub cc-ltr">{d.url}</div>
            </div>
            <button type="button" className="cc-rowbtn" aria-label="کپی آدرس" onClick={async () => {
              const ok = await fallbackCopy(d.url)
              setNotice(ok ? 'آدرس کپی شد' : d.url)
            }}><Copy size={15} /></button>
          </div>
          <div className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">تست سلامت</div>
              <div className="cc-sub cc-ltr">{d.health || '—'}</div>
            </div>
            <Badge variant={d.serverUp ? 'success' : 'error'}>{d.serverUp ? 'OK ' + faNum(d.probeCode) : 'خطا'}</Badge>
          </div>
          <div className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">اکانت کلادفلر</div>
              <div className="cc-sub cc-ltr">{d.account || '—'}</div>
            </div>
          </div>
          <div className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">فضای KV</div>
              <div className="cc-sub cc-ltr">{d.namespace || '—'}</div>
            </div>
          </div>

          <div className="cc-btnrow cc-mt">
            <Button variant="outline" icon={ArrowClockwise} disabled={!!busy} onClick={() => act('/api/tools/fo/refresh', {}, 'داده‌های ورکر تازه شد')}>
              تازه‌سازی داده
            </Button>
            <Button variant="outline" icon={Power} disabled={!!busy} onClick={() => act(d.enabled ? '/api/tools/fo/disable' : '/api/tools/fo/enable', {}, d.enabled ? 'فیلور خاموش شد' : 'فیلور روشن شد')}>
              {d.enabled ? 'غیرفعال کردن' : 'فعال کردن'}
            </Button>
            <Button variant="outline" icon={Trash} disabled={!!busy} onClick={() => setDel(true)}>
              حذف کامل
            </Button>
          </div>
          <div className="cc-hint cc-mt">
            کرون ورکر هر ۵ دقیقه سلامت سرور را چک می‌کند؛ پس از دو شکست پیاپی، وبهوک را به خودش می‌برد و به‌محض برگشت سرور پس می‌دهد.
          </div>
        </div>
      )}

      {d && !d.configured && (
        <Empty title="فیلور تنظیم نشده" description="با توکن کلادفلر (DNS + Workers) یک ورکر پشتیبان ساخته می‌شود تا هنگام قطعی سرور، ربات از کلادفلر جواب دهد." />
      )}

      {form && (
        <FormDialog
          title={d?.configured ? 'تنظیم مجدد فیلور' : 'راه‌اندازی فیلور'}
          sub="توکن باید دسترسی Workers Scripts و KV و DNS داشته باشد."
          onClose={() => setForm(null)}
          error={fErr}
          onSave={async () => {
            setFErr('')
            try {
              await save('/api/tools/fo/setup', { token: form.token.trim(), name: form.name.trim() }, () => {})
              setForm(null)
              setNotice('در حال ساخت ورکر… نتیجه در تلگرام هم می‌آید')
              reload()
            } catch (e: any) {
              setFErr(String(e?.message || e))
            }
          }}
        >
          <label className="cc-field">
            <span>توکن کلادفلر</span>
            <Input type="password" autoComplete="off" value={form.token} onChange={(e) => setForm({ ...form, token: e.target.value })} placeholder="cf token…" />
          </label>
          <label className="cc-field">
            <span>نام ورکر (اختیاری)</span>
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="cg-failover" />
          </label>
        </FormDialog>
      )}

      {del && (
        <Dialog.Root open onOpenChange={() => setDel(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف کامل فیلور</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              اسکریپت ورکر، فضای KV و رکورد سلامت حذف می‌شوند. بعد از آن در صورت قطعی سرور، ربات خاموش می‌ماند.
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="outline" onClick={() => setDel(false)}>انصراف</Button>
              <Button variant="destructive" onClick={async () => { setDel(false); await act('/api/tools/fo/delete', {}, 'فیلور حذف شد') }}>حذف کن</Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
    </>
  )
}

export default FoView
