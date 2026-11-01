import { useEffect, useState } from 'react'
import { CaretLeft, Lightning, Envelope, GearSix, Lifebuoy, Broadcast, ArrowsClockwise, CalendarDots, BellSimple, Fire, GlobeSimple, Pulse, ArrowClockwise, BellRinging, Check, Copy, Gear, HardDrive, Info, PencilSimple, Plus, Power, ShieldCheck, Terminal, Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { StatGrid } from '../components/Stats'
import { FilterBar } from '../components/FilterBar'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, save, faTxt } from './shared'
import type { Srv, Rem, PgPanel, PgMon, PgData } from './shared'

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
  const num = (k: keyof UmCfg, label: string, hint: string) => (
    <label className="cc-field">
      <span>{label}</span>
      <Input
        type="number"
        value={String(cfg[k])}
        onChange={(e) => setCfg({ ...cfg, [k]: Number(e.target.value) || 0 })}
        placeholder={hint}
      />
    </label>
  )
  const submit = async () => {
    setFErr('')
    try {
      await save('/api/tools/um', cfg, () => setNotice('ذخیره شد'))
    } catch (e: any) {
      setFErr(e.message || 'ذخیره نشد')
    }
  }
  return (
    <>
      <StatGrid
        items={[
          { label: 'آستانهٔ ساعتی', value: faNum(cfg.hourlyGb) + ' GB' },
          { label: 'آستانهٔ روزانه', value: faNum(cfg.dailyGb) + ' GB' },
          { label: 'حداقل اسنپ‌شات', value: faNum(cfg.minGb) + ' GB' },
        ]}
      />
      <div className="cc-hint cc-mb">
        کاربرانی که لینکشان پخش شده و بی‌رویه مصرف می‌کنند را پیدا می‌کند؛ هر ۶۰ دقیقه مصرف فعلی را با ~۱ ساعت و ~۲۴ ساعت قبل مقایسه می‌کند و هشدار را فقط برای ادمین می‌فرستد.
      </div>
      <div className="cc-form">
        <label className="cc-checkline">
          <input id="cgc2" name="cgc2" type="checkbox" checked={!!cfg.enabled} onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })} />
          <span>پایش بد مصرف فعال باشد</span>
        </label>
        <label className="cc-checkline">
          <input id="cgc3" name="cgc3" type="checkbox" checked={!!cfg.report} onChange={(e) => setCfg({ ...cfg, report: e.target.checked })} />
          <span>گزارش روزانهٔ اخطارشده‌ها ارسال شود</span>
        </label>
        {num('hourlyGb', 'آستانهٔ ساعتی (GB)', '7')}
        {num('dailyGb', 'آستانهٔ روزانه (GB)', '25')}
        {num('cooldownH', 'فاصلهٔ تکرار هشدار هر کاربر (ساعت)', '6')}
        {num('minGb', 'حداقل مصرف برای اسنپ‌شات (GB)', '1')}
        {num('activeMin', 'حداقل مدت فعال بودن (دقیقه)', '75')}
      </div>
      <div className="cc-pagebar">
        <Button variant="primary" onClick={submit}>
          ذخیره
        </Button>
      </div>
      {fErr && <div className="cc-form-err cc-center">{fErr}</div>}
      {notice && <div className="cc-hint cc-center">{notice}</div>}
    </>
  )
}

export default UmView
