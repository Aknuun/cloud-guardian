import { useEffect, useState } from 'react'
import { CaretLeft, Lightning, Envelope, GearSix, Lifebuoy, Broadcast, ArrowsClockwise, CalendarDots, BellSimple, Fire, GlobeSimple, Pulse, ArrowClockwise, BellRinging, Check, Copy, Gear, HardDrive, Info, PencilSimple, Plus, Power, ShieldCheck, Terminal, Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { StatGrid } from '../components/Stats'
import { FilterBar } from '../components/FilterBar'
import { usePageAction } from '../components/PageAction'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, save, faTxt } from './shared'
import type { Srv, Rem, PgPanel, PgMon, PgData } from './shared'

function RemView() {
  usePageAction({ label: 'یادآور جدید', icon: Plus, onClick: () => setAdd(true) })

  const { data, err, loaded, reload } = useTool<Rem[]>('/api/tools/rem')
  const [add, setAdd] = useState(false)
  const [text, setText] = useState('')
  const [when, setWhen] = useState('')
  const [rel, setRel] = useState(0)
  const [back, setBack] = useState(false)
  const [note, setNote] = useState('')
  const [server, setServer] = useState('')
  const [formErr, setFormErr] = useState('')
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')
  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!loaded) return <Skeleton />
  const list = data || []
  const pending = list.filter((r) => !r.ack).length
  const needle = q.trim().toLowerCase()
  const shown = list
    .filter((r) => (flt === 'pending' ? !r.ack : flt === 'done' ? r.ack : true))
    .filter((r) => !needle || (r.text + ' ' + r.server).toLowerCase().includes(needle))

  const submit = async () => {
    setFormErr('')
    try {
      await save('/api/tools/rem/add', { text, when: back ? '' : when, server, note, rel: back ? 0 : rel }, () => {
        setAdd(false)
        setText('')
        setWhen('')
        setNote('')
        setRel(0)
        setBack(false)
        setServer('')
        reload()
      })
    } catch (e: any) {
      setFormErr(e.message || 'ذخیره نشد')
    }
  }

  return (
    <>
      <StatGrid
        items={[
          { label: 'یادآورها', value: faNum(list.length) },
          { label: 'در انتظار', value: faNum(pending) },
          { label: 'انجام‌شده', value: faNum(list.length - pending) },
        ]}
      />
      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی یادآورها"
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'pending', label: 'در انتظار' },
          { id: 'done', label: 'انجام‌شده' },
        ]}
        value={flt}
        onValue={setFlt}
        onRefresh={() => {
          invalidateCache()
          reload()
        }}
      />
      {shown.length === 0 && (
        <Empty title="یادآوری نیست" description={needle ? 'چیزی با این جستجو پیدا نشد.' : 'با دکمهٔ «یادآور جدید» یکی بساز.'} />
      )}
      {shown.map((r) => (
        <div key={r.id} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{r.text}</div>
            <div className="cc-sub">
              {r.when > 0 ? faDate(Number(r.when)) : 'هر وقت آنلاین شدی'}
              {r.server ? ' · ' + r.server : ''}
            </div>
            {r.note ? <div className="cc-sub">{r.note}</div> : null}
          </div>
          <Badge>{r.ack ? 'انجام شد' : r.notified ? 'اطلاع داده شد' : 'در انتظار'}</Badge>
        </div>
      ))}

      {add && (
        <FormDialog title="یادآور جدید" onClose={() => setAdd(false)} onSave={submit} error={formErr}>
          <label className="cc-field">
            <span>متن</span>
            <Input value={text} onChange={(e) => setText(e.target.value)} placeholder="تمدید سرور آلمان" />
          </label>
          <div className="cc-field">
            <span>زمان سریع</span>
            <div className="cc-chips">
              {[[15, '۱۵ دقیقه'], [60, '۱ ساعت'], [180, '۳ ساعت'], [1440, '۱ روز'], [10080, '۱ هفته']].map(([m, l]) => (
                <button
                  key={m}
                  type="button"
                  className={'cc-chip' + (rel === m ? ' on' : '')}
                  onClick={() => setRel(rel === (m as number) ? 0 : (m as number))}
                >
                  {l}
                </button>
              ))}
            </div>
          </div>
          <label className="cc-field">
            <span>یا تاریخ و ساعت (شمسی)</span>
            <Input
              value={when}
              onChange={(e) => setWhen(e.target.value)}
              placeholder="1404/08/07 14:30"
              dir="ltr"
              disabled={rel > 0 || back}
            />
          </label>
          <label className="cc-checkline">
            <input id="cgc1" name="cgc1" type="checkbox" checked={back} onChange={(e) => setBack(e.target.checked)} />
            <span>وقتی سرور برگشت آنلاین شد یادم بنداز</span>
          </label>
          <label className="cc-field">
            <span>یادداشت (اختیاری)</span>
            <Input value={note} onChange={(e) => setNote(e.target.value)} placeholder="یادداشت" />
          </label>
          <label className="cc-field">
            <span>سرور (اختیاری)</span>
            <Input value={server} onChange={(e) => setServer(e.target.value)} placeholder="usa-1" dir="ltr" />
          </label>
        </FormDialog>
      )}
    </>
  )
}

export default RemView
