import { useEffect, useState } from 'react'
import { Copy, } from '@phosphor-icons/react'
import { api, fallbackCopy, invalidateCache } from '../lib'
import { Button, Empty, Input, } from '../components/ui'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, } from './shared'

type HookPanel = {
  idx: number; name: string; url: string; set: boolean
  secret?: string; hook?: string; dashpath?: string; renewbot?: string; renewbotAuto?: boolean
}
type HookData = { enabled: boolean; silentMe: boolean; days: number[]; usage: number[]; panels: HookPanel[] }

function HookView() {
  const { data, err, reload } = useTool<HookData>('/api/tools/hook')
  const [d, setD] = useState<HookData | null>(null)
  const [edit, setEdit] = useState<HookPanel | null>(null)
  const [dash, setDash] = useState('')
  const [rb, setRb] = useState('')
  const [auto, setAuto] = useState(false)
  const [fErr, setFErr] = useState('')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    if (data) setD(data)
  }, [data])

  async function act(path: string, body: any, msg?: string) {
    setFErr('')
    try {
      await api(path, body)
      invalidateCache()
      if (msg) setNotice(msg)
      reload()
      return true
    } catch (e: any) {
      setFErr(e.message || 'انجام نشد')
      return false
    }
  }
  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!d)
    return (
      <Skeleton rows={5} />
    )
  return (
    <>
      <div className="cc-hint cc-mb">
        آدرس وبهوک هر پنل را در تنظیمات webhook پنل (بخش اعلان‌ها) بگذار؛ خود پنل وقتی کاربری به آستانهٔ انقضا یا حجم رسید خبر
        می‌دهد. آستانه‌ها (days_left و usage_percent) هم همان‌جا در پنل تنظیم می‌شوند.
      </div>
      <div className="cc-form">
        <label className="cc-checkline">
          <input id="cgc5" name="cgc5" type="checkbox" checked={!!d.enabled} onChange={(e) => act('/api/tools/hook/cfg', { enabled: e.target.checked })} />
          <span>هشدار انقضا و حجم فعال باشد</span>
        </label>
        <label className="cc-checkline">
          <input id="cgc6" name="cgc6" type="checkbox" checked={!!d.silentMe} onChange={(e) => act('/api/tools/hook/cfg', { silentMe: e.target.checked })} />
          <span>بی‌صدا: هشدارها به خود کاربران برود، نه به من</span>
        </label>
        <div className="cc-field-row">
          <label className="cc-field">
            <span>آستانهٔ روز (با کاما)</span>
            <Input
              value={(d.days || []).join(', ')}
              onChange={(e) => setD({ ...d, days: e.target.value.split(',').map((x) => Number(x.trim()) || 0).filter((x) => x > 0) })}
            />
          </label>
          <label className="cc-field">
            <span>آستانهٔ حجم ٪ (با کاما)</span>
            <Input
              value={(d.usage || []).join(', ')}
              onChange={(e) => setD({ ...d, usage: e.target.value.split(',').map((x) => Number(x.trim()) || 0).filter((x) => x > 0) })}
            />
          </label>
        </div>
        {fErr && <div className="cc-form-err">{fErr}</div>}
        <Button variant="primary" onClick={() => act('/api/tools/hook/cfg', { days: d.days, usage: d.usage }, 'ذخیره شد')}>
          ذخیرهٔ آستانه‌ها
        </Button>
      </div>
      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>پنل‌ها</h2>
        </div>
        {d.panels.length === 0 && <Empty title="پنلی نیست" description="اول در بخش پاسارگارد یک پنل اضافه کن؛ بعد اینجا هشدارها را تنظیم کن." />}
        {d.panels.map((p) => (
          <div key={p.idx} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">{p.name}</div>
              <div className="cc-sub cc-ltr">{p.hook || 'وبهوک ساخته نشده'}</div>
              {p.dashpath && <div className="cc-sub cc-ltr">dashboard: {p.dashpath}</div>}
              {p.renewbot && (
                <div className="cc-sub cc-ltr">
                  renew-bot: @{p.renewbot}
                  {p.renewbotAuto ? ' (خودکار)' : ''}
                </div>
              )}
            </div>
            {!p.set ? (
              <Button variant="primary" onClick={() => act('/api/tools/hook/panel', { idx: p.idx }, 'وبهوک ساخته شد')}>
                ساخت وبهوک
              </Button>
            ) : (
              <div className="cc-btnrow">
                <Button
                  variant="primary"
                  onClick={() => act('/api/tools/hook/apply', { idx: p.idx }, 'در تنظیمات پنل ثبت شد')}
                >
                  ست کردن خودکار
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setEdit(p)
                    setDash(p.dashpath || '')
                    setRb(p.renewbot || '')
                    setAuto(!!p.renewbotAuto)
                    setFErr('')
                  }}
                >
                  تنظیمات
                </Button>
                <Button variant="outline" onClick={() => act('/api/tools/hook/test', { idx: p.idx }, 'پیام تستی به تلگرام رفت')}>
                  پیام تستی
                </Button>
              </div>
            )}
          </div>
        ))}
      </div>

      {edit && (
        <FormDialog
          title="تنظیم وبهوک"
          sub={edit.name}
          onClose={() => setEdit(null)}
          onSave={() =>
            act(
              '/api/tools/hook/panel',
              { idx: edit.idx, dashpath: dash, renewbot: rb, renewbotAuto: auto },
              'ذخیره شد',
            ).then((ok) => ok && setEdit(null))
          }
          error={fErr}
        >
          {edit.hook && (
            <div className="cc-field">
              <span>آدرس وبهوک</span>
              <div className="cc-pre" style={{ whiteSpace: 'normal' }}>{edit.hook}</div>
              <div className="cc-btnrow">
                <Button
                  variant="outline"
                  icon={Copy}
                  onClick={() => {
                    const v = edit.hook || ''
                    fallbackCopy(v).then((ok) => setNotice(ok ? 'آدرس کپی شد' : 'کپی نشد؛ دستی انتخاب کن: ' + v))
                  }}
                >
                  کپی آدرس
                </Button>
                <Button
                  variant="outline"
                  icon={Copy}
                  onClick={() => {
                    const v = edit.secret || ''
                    fallbackCopy(v).then((ok) => setNotice(ok ? 'سکرت کپی شد' : 'کپی نشد؛ سکرت: ' + v))
                  }}
                >
                  کپی سکرت
                </Button>
              </div>
              <div className="cc-hint">با «ست کردن خودکار» همین آدرس و سکرت مستقیم در تنظیمات پنل ذخیره می‌شود.</div>
              <Button variant="primary" onClick={() => act('/api/tools/hook/apply', { idx: edit.idx }, 'در تنظیمات پنل ثبت شد')}>
                ست کردن خودکار در پنل
              </Button>
            </div>
          )}
          <label className="cc-field">
            <span>مسیر داشبورد (خالی = /dashboard)</span>
            <Input value={dash} onChange={(e) => setDash(e.target.value)} placeholder="/dashboard" />
          </label>
          <label className="cc-field">
            <span>ربات داخلی تمدید (یوزرنیم بدون @)</span>
            <Input value={rb} onChange={(e) => setRb(e.target.value)} placeholder="RenewBot" />
          </label>
          <label className="cc-checkline">
            <input id="cgc7" name="cgc7" type="checkbox" checked={auto} onChange={(e) => setAuto(e.target.checked)} />
            <span>تمدید خودکار با ربات داخلی</span>
          </label>
          <Button
            variant="outline"
            onClick={() =>
              act('/api/tools/hook/panel', { idx: edit.idx, regen: true }, 'توکن جدید ساخته شد').then((ok) => {
                if (ok) setEdit(null)
              })
            }
          >
            ساخت توکن جدید (سکرت قبلی باطل می‌شود)
          </Button>
        </FormDialog>
      )}
    </>
  )
}

// ---------------- daily sub rotation ----------------

export default HookView
