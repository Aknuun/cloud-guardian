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

type HubStats = Partial<{ bot: string; version: string; go: string; uptime: string; mem_mb: number; db_mb: number; admins: number; users: number; panels: number; zones: number; cf_accs: number; servers: number }>

// stats come from older/newer peers; never let a missing field crash the page
const sv = (v?: string) => (v && String(v).trim() ? v : '—')
const sn = (v?: number) => (typeof v === 'number' && Number.isFinite(v) ? faNum(v) : '—')
const sf = (v: number | undefined, d: number) => (typeof v === 'number' && Number.isFinite(v) ? v.toFixed(d) : '—')
type HubData = { hubUrl: string; annUrl: string; secret: boolean; root?: boolean; stats: HubStats; hubOk: boolean; hubErr?: string; installs: any[] }
type Ann = { id: string; text: string; until: string; reply: boolean }


type HubOut = { id: string; text: string; ts: number; target: string; members: number; delivered: number }
type HubInst = { name: string; bot: string; version: string; feed: number; seen: number; users: number; servers: number; zones: number; ip: string }

function HubHistory() {
  const [items, setItems] = useState<HubOut[]>([])
  const [insts, setInsts] = useState<HubInst[]>([])
  const [err, setErr] = useState('')
  const load = () =>
    api<{ items: HubOut[]; installs: HubInst[] }>(`/api/tools/hub/history?t=${Date.now()}`)
      .then((d) => { setItems(d.items || []); setInsts(d.installs || []) })
      .catch((e: any) => setErr(String(e?.message || e)))
  useEffect(() => { load() }, [])
  const del = async (id: string) => {
    await api('/api/tools/hub/history/del', { id })
    load()
  }
  const shown = items.slice().reverse()
  return (
    <div className="cc-section">
      <div className="cc-section-head">
        <h2>تاریخچهٔ پیام‌ها</h2>
        <Button variant="outline" icon={ArrowClockwise} onClick={load}>تازه‌سازی</Button>
      </div>
      {err && <ErrBox msg={err} />}
      {shown.length === 0 && <Empty title="پیامی ثبت نشده" description="هر پیامی که بفرستی اینجا با تعداد دریافت‌کننده‌ها می‌آید." />}
      {shown.map((h) => (
        <div key={h.id} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{h.text}</div>
            <div className="cc-sub">
              {h.target === 'installs'
                ? `${faNum(h.delivered)} از ${faNum(h.members)} نصب دریافت کردند`
                : `برای ${faNum(h.members)} نفر ارسال شد`}{' · '}{faDate(h.ts)}
            </div>
          </div>
          <Badge variant={h.target === 'installs' ? 'primary' : 'success'}>{h.target === 'installs' ? 'نصب‌ها' : 'کاربران'}</Badge>
          <button type="button" className="cc-rowbtn danger" aria-label="حذف" onClick={() => del(h.id)}>
            <Trash size={15} />
          </button>
        </div>
      ))}
      {insts.length > 0 && (
        <>
          <div className="cc-sub" style={{ margin: '12px 2px 6px' }}>وضعیت نصب‌های عضو</div>
          {insts.map((m, i) => (
            <div key={i} className="cc-trow">
              <div className="cc-grow">
                <div className="cc-name">{m.name || m.bot}</div>
                <div className="cc-sub">
                  {m.feed > 0 ? 'آخرین دریافت: ' + faDate(m.feed) : 'هنوز پیامی دریافت نکرده'} · {faNum(m.users)} کاربر
                </div>
              </div>
              <Badge variant={m.seen > Date.now() - 15 * 60000 ? 'success' : 'error'}>
                {m.seen > Date.now() - 15 * 60000 ? 'آنلاین' : 'ساکت'}
              </Badge>
            </div>
          ))}
        </>
      )}
    </div>
  )
}

function HubView() {
  const { data, err, loaded, reload } = useTool<HubData>('/api/tools/hub')
  const [anns, setAnns] = useState<Ann[] | null>(null)
  const [annErr, setAnnErr] = useState('')
  const [cfg, setCfg] = useState<{ hubUrl: string; annUrl: string; secret: string } | null>(null)
  const [cErr, setCErr] = useState('')
  const [cast, setCast] = useState<string | null>(null)
  const [castAll, setCastAll] = useState<string | null>(null)
  const [notice, setNotice] = useState('')

  useEffect(() => {
    api<{ items: Ann[] }>('/api/tools/hub/ann')
      .then((d) => setAnns(d.items || []))
      .catch((e) => setAnnErr(String(e?.message || e)))
  }, [])

  const s = data?.stats

  return (
    <>
      <StatGrid
        items={[
          { label: 'نسخه', value: sv(s?.version) },
          { label: 'آپ‌تایم', value: sv(s?.uptime) },
          { label: 'نصب‌های هاب', value: data?.installs?.length ? faNum(data.installs.length) : '—' },
        ]}
      />

      <div className="cc-pagebar">
        <Button variant="primary" icon={GearSix} onClick={() => { setCErr(''); setCfg({ hubUrl: data?.hubUrl || '', annUrl: data?.annUrl || '', secret: '' }) }}>
          تنظیم هاب
        </Button>
      </div>

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}
      {err && <ErrBox msg={err} onRetry={reload} />}
      {!loaded && <Skeleton rows={3} />}

      {s && (
        <div className="cc-section">
          <div className="cc-section-head"><h2>این نصب</h2></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">ربات</div><div className="cc-sub">{(s.bot || '—').startsWith('@') ? (s.bot || '—') : '@' + (s.bot || '—')}</div></div></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">کاربران / ادمین‌ها</div><div className="cc-sub">{sn(s.users)} / {sn(s.admins)}</div></div></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">پنل‌ها / سرورها</div><div className="cc-sub">{sn(s.panels)} / {sn(s.servers)}</div></div></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">دامنه‌ها</div><div className="cc-sub">{sn(s.zones)} در {sn(s.cf_accs)} اکانت</div></div></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">حافظه / دیتابیس</div><div className="cc-sub">{sf(s.mem_mb, 1)} MB / {sf(s.db_mb, 2)} MB</div></div></div>
        </div>
      )}

      {data && (data.hubUrl || data.root) && (
        <div className="cc-section">
          <div className="cc-section-head"><h2>نصب‌های هاب</h2></div>
          {!data.hubOk && <div className="cc-hint cc-center cc-mb">{data.hubErr || 'هاب در دسترس نیست'}</div>}
          {data.installs.map((x: any, i: number) => (
            <div key={i} className="cc-trow">
              <div className="cc-grow">
                <div className="cc-name">{x.bot || x.name || '—'}</div>
                <div className="cc-sub">{(x.version ? 'v' + x.version + ' · ' : '')}{faNum(x.users || 0)} کاربر · {faNum(x.servers || 0)} سرور</div>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>اعلان‌ها</h2>
          <Button variant="outline" icon={Plus} onClick={() => setCast('')}>پیام به کاربرانم</Button>
          {data?.root && <Button variant="outline" icon={Plus} onClick={() => setCastAll('')}>پیام به نصب‌ها</Button>}
        </div>
        {annErr && <div className="cc-hint cc-center cc-mb">{annErr}</div>}
        {anns && anns.length === 0 && <Empty title="اعلانی نیست" description="از منبع اعلان‌ها چیزی برنگشت." />}
        {(anns || []).map((a) => (
          <div key={a.id} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">{a.text}</div>
              <div className="cc-sub">{a.until ? 'تا ' + faDate(a.until) : 'بدون پایان'}{a.reply ? ' · با پاسخ' : ''}</div>
            </div>
          </div>
        ))}
      </div>

      {cfg && (
        <FormDialog
          title="تنظیم هاب"
          sub="آدرس هاب برای تجمیع آمار، و آدرس منبع اعلان‌ها (JSON)."
          onClose={() => setCfg(null)}
          error={cErr}
          onSave={async () => {
            setCErr('')
            try {
              await save('/api/tools/hub/cfg', cfg, () => {})
              setCfg(null)
              setNotice('تنظیمات ذخیره شد')
              reload()
            } catch (e: any) {
              setCErr(String(e?.message || e))
            }
          }}
        >
          <label className="cc-field"><span>آدرس هاب</span><Input className="cc-ltr" value={cfg.hubUrl} onChange={(e) => setCfg({ ...cfg, hubUrl: e.target.value })} placeholder="https://hub.example.com" /></label>
          <label className="cc-field"><span>آدرس اعلان‌ها</span><Input className="cc-ltr" value={cfg.annUrl} onChange={(e) => setCfg({ ...cfg, annUrl: e.target.value })} placeholder="https://…/announcements.json" /></label>
          <label className="cc-field"><span>کلید هاب (اختیاری)</span><Input type="password" value={cfg.secret} onChange={(e) => setCfg({ ...cfg, secret: e.target.value })} placeholder="برای تغییر نکردن خالی بگذار" /></label>
        </FormDialog>
      )}

      {cast !== null && (
        <FormDialog
          title="پیام به کاربران من"
          sub="برای همهٔ ادمین‌ها و کاربران شناخته‌شده ارسال می‌شود."
          onClose={() => setCast(null)}
          onSave={async () => {
            try {
              const r = await api<{ queued: number }>('/api/tools/hub/cast', { text: cast })
              setCast(null)
              setNotice(`در صف ارسال برای ${faNum(r.queued)} نفر`)
            } catch (e: any) {
              setNotice(String(e?.message || e))
            }
          }}
        >
          <label className="cc-field">
            <span>متن پیام</span>
            <textarea className="cc-input cc-textarea" value={cast} onChange={(e) => setCast(e.target.value)} />
          </label>
        </FormDialog>
      )}
      {castAll !== null && (
        <FormDialog
          title="پیام به نصب‌ها"
          sub="این پیام در فید هاب می‌رود و هر نصب عضو آن را برای ادمین خودش می‌فرستد."
          onClose={() => setCastAll(null)}
          onSave={async () => {
            try {
              const r = await api<{ members: number }>('/api/tools/hub/cast-installs', { text: castAll })
              setCastAll(null)
              setNotice(`پیام برای ${faNum(r.members)} نصب در فید هاب ثبت شد`)
            } catch (e: any) {
              setNotice(String(e?.message || e))
            }
          }}
        >
          <label className="cc-field">
            <span>متن پیام</span>
            <textarea className="cc-input cc-textarea" value={castAll} onChange={(e) => setCastAll(e.target.value)} />
          </label>
        </FormDialog>
      )}
      <HubHistory />
    </>
  )
}

export default HubView
