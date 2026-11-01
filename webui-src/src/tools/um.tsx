import { useEffect, useState } from 'react'
import { api, } from '../lib'
import { Button, Input, Switch } from '../components/ui'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { save, } from './shared'
import { SettingsCard, SettingsRow } from '../components/ui-kit'

type UmCfg = {
  enabled: boolean
  report: boolean
  hourlyGb: number
  dailyGb: number
  cooldownH: number
  activeMin: number
  minGb: number
}

function UmView() {
  const [cfg, setCfg] = useState<UmCfg | null>(null)
  const [err, setErr] = useState('')
  const [notice, setNotice] = useState('')
  const [fErr, setFErr] = useState('')
  const [saved, setSaved] = useState(false)
  const load = () => {
    setErr('')
    api<UmCfg>('/api/tools/um')
      .then(setCfg)
      .catch((e) => setErr(e.message || 'خطا'))
  }
  // biome-ignore lint/correctness/useExhaustiveDependencies: load is stable enough here
  useEffect(load, [])
  if (err) return <ErrBox msg={err} onRetry={load} />
  if (!cfg)
    return (
      <Skeleton rows={5} />
    )
  const num = (k: keyof UmCfg, label: string, unit: string, hint: string) => (
    <SettingsRow label={label}>
      <Input
        className="cc-num"
        type="number"
        value={String(cfg[k])}
        onChange={(e) => setCfg({ ...cfg, [k]: Number(e.target.value) || 0 })}
        placeholder={hint}
        aria-label={label}
      />
      <span className="cc-unit">{unit}</span>
    </SettingsRow>
  )
  const submit = async () => {
    setFErr('')
    setSaved(false)
    try {
      await save('/api/tools/um', cfg, () => {})
      setSaved(true)
      setNotice('')
    } catch (e: any) {
      setFErr(e.message || 'ذخیره نشد')
    }
  }
  return (
    <>
      <div className="cc-hint cc-mb">
        کاربرانی که لینکشان پخش شده و بی‌رویه مصرف می‌کنند را پیدا می‌کند؛ هر ۶۰ دقیقه مصرف فعلی را با ~۱ ساعت و ~۲۴ ساعت قبل مقایسه می‌کند و هشدار را فقط برای ادمین می‌فرستد.
      </div>
      <SettingsCard
        title="تنظیمات پایش"
        footer={
          <>
            <Button variant="primary" onClick={submit}>ذخیره</Button>
            {fErr ? <span className="cc-form-err">{fErr}</span> : saved ? <span className="cc-hint">ذخیره شد</span> : null}
          </>
        }
      >
        <SettingsRow label="پایش بد مصرف">
          <Switch checked={!!cfg.enabled} onCheckedChange={(v) => setCfg({ ...cfg, enabled: v })} aria-label="پایش بد مصرف فعال باشد" />
        </SettingsRow>
        <SettingsRow label="گزارش روزانهٔ اخطارشده‌ها">
          <Switch checked={!!cfg.report} onCheckedChange={(v) => setCfg({ ...cfg, report: v })} aria-label="گزارش روزانه ارسال شود" />
        </SettingsRow>
        {num('hourlyGb', 'آستانهٔ ساعتی', 'GB', '7')}
        {num('dailyGb', 'آستانهٔ روزانه', 'GB', '25')}
        {num('cooldownH', 'فاصلهٔ تکرار هشدار', 'ساعت', '6')}
        {num('minGb', 'حداقل مصرف برای اسنپ‌شات', 'GB', '1')}
        {num('activeMin', 'حداقل مدت فعال بودن', 'دقیقه', '75')}
      </SettingsCard>
      {notice && <div className="cc-hint cc-center">{notice}</div>}
    </>
  )
}

export default UmView
