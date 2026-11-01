import { useEffect, useState } from 'react'
import { Copy, } from '@phosphor-icons/react'
import { api, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Button, Empty, Input, Switch } from '../components/ui'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { SectionHeader } from '../components/SectionHeader'
import { FormDialog } from './shared'
import { SettingsCard, SettingsRow } from '../components/ui-kit'
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
  const [saved, setSaved] = useState(false)

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
      <SettingsCard
        title="تنظیمات هشدار"
        action={
          <Switch checked={!!d.enabled} onCheckedChange={(v) => act('/api/tools/hook/cfg', { enabled: v })} aria-label="هشدار انقضا و حجم فعال باشد" />
        }
        footer={
          <>
            <Button
              variant="primary"
              onClick={async () => {
                setSaved(false)
                if (await act('/api/tools/hook/cfg', { days: d.days, usage: d.usage })) setSaved(true)
              }}
            >
              ذخیرهٔ آستانه‌ها
            </Button>
            {fErr ? <span className="cc-form-err">{fErr}</span> : saved ? <span className="cc-hint">ذخیره شد</span> : null}
          </>
        }
      >
        <SettingsRow label="بی‌صدا" hint="هشدارها به خود کاربران برود، نه به من">
          <Switch checked={!!d.silentMe} onCheckedChange={(v) => act('/api/tools/hook/cfg', { silentMe: v })} aria-label="ارسال بی‌صدا به کاربران" />
        </SettingsRow>
        <div className="cc-setgrid">
          <label className="cc-setfield">
            <span>
              آستانهٔ روز <i>(با کاما)</i>
            </span>
            <Input
              className="cc-num"
              aria-label="آستانهٔ روز"
              value={(d.days || []).join(', ')}
              onChange={(e) => setD({ ...d, days: e.target.value.split(',').map((x) => Number(x.trim()) || 0).filter((x) => x > 0) })}
            />
          </label>
          <label className="cc-setfield">
            <span>
              آستانهٔ حجم <i>(٪ با کاما)</i>
            </span>
            <Input
              className="cc-num"
              aria-label="آستانهٔ حجم"
              value={(d.usage || []).join(', ')}
              onChange={(e) => setD({ ...d, usage: e.target.value.split(',').map((x) => Number(x.trim()) || 0).filter((x) => x > 0) })}
            />
          </label>
        </div>
      </SettingsCard>
      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <SectionHeader title="پنل‌ها" count={faNum(d.panels.length) + ' پنل'} />
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
              <div className="cc-actions">
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
              <div className="cc-actions">
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
