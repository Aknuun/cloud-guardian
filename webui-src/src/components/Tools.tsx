import { useEffect, useState } from 'react'
import { CaretLeft, Lightning, Envelope, GearSix, Lifebuoy, Broadcast, ArrowsClockwise, CalendarDots, BellSimple, Fire, GlobeSimple, Pulse, ArrowClockwise, BellRinging, Check, Copy, Gear, HardDrive, Info, PencilSimple, Plus, Power, ShieldCheck, Terminal, Trash } from '@phosphor-icons/react'
import { api, faDate, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from './ui'
import { StatGrid } from './Stats'
import { FilterBar } from './FilterBar'
import { TOOLS, FormDialog, HomeView, ErrBox, Skeleton } from './toolsMeta'
import type { ToolId } from './toolsMeta'
import type { Provider } from './Sidebar'


function useTool<T>(path: string) {
  const [data, setData] = useState<T | null>(null)
  const [err, setErr] = useState('')
  const [loaded, setLoaded] = useState(false)
  const [nonce, setNonce] = useState(0)
  useEffect(() => {
    let stop = false
    api<T>(path)
      .then((d) => {
        if (stop) return
        setData(d)
        setLoaded(true)
      })
      .catch((e) => {
        if (stop) return
        setErr(e.message || 'خطا')
        setLoaded(true)
      })
    return () => {
      stop = true
    }
  }, [path, nonce])
  return { data, err, loaded, reload: () => { setNonce((n) => n + 1) } }
}

// POST + refresh the section; the GET cache is dropped first so the list is fresh.
async function save(path: string, body: any, done: () => void) {
  await api(path, body)
  invalidateCache()
  done()
}

type Srv = { id: string; name: string; host: string; user: string; port: number; note: string; key: boolean; secret?: string }
type Rem = { id: string; text: string; when: number; notified: boolean; ack: boolean; server: string; note?: string }
type PgPanel = { idx: number; name: string; url: string; user: string }
type PgMon = { id: string; name: string; panel: string; interval: number; enabled: boolean }
type PgData = { panels: PgPanel[]; monitors: PgMon[] }

export function Tools({ id, onGo }: { id: ToolId; onGo?: (p: Provider | null, t: ToolId | null) => void }) {
  if (id === 'home') return <HomeView go={onGo} />
  if (id === 'srv') return <SrvView />
  if (id === 'rem') return <RemView />
  if (id === 'um') return <UmView />
  if (id === 'dom') return <DomView />
  if (id === 'hf') return <HfView />
  if (id === 'hfd') return <HfdView />
  if (id === 'hook') return <HookView />
  if (id === 'mail') return <MailView />
  if (id === 'fo') return <FoView />
  if (id === 'hub') return <HubView />
  if (id === 'set') return <SetView />
  return <PgView />
}

function SrvView() {
  const { data, err, loaded, reload } = useTool<Srv[]>('/api/tools/srv')
  const [edit, setEdit] = useState<Partial<Srv> | null>(null)
  const [del, setDel] = useState<Srv | null>(null)
  const [formErr, setFormErr] = useState('')
  const [mon, setMon] = useState<Srv | null>(null)
  const [monData, setMonData] = useState<any>(null)
  const [monErr, setMonErr] = useState('')
  const [monBusy, setMonBusy] = useState(false)
  const [act, setAct] = useState<'reboot' | 'upd' | null>(null)
  const [actSrv, setActSrv] = useState<Srv | null>(null)
  const [notice, setNotice] = useState('')
  const [detail, setDetail] = useState<Srv | null>(null)
  const [info, setInfo] = useState<{ s: Srv; kind: 'info' | 'disk' } | null>(null)
  const [cfgOpen, setCfgOpen] = useState(false)
  const [impOpen, setImpOpen] = useState(false)
  const [pwOpen, setPwOpen] = useState(false)
  const [subOpen, setSubOpen] = useState(false)
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')
  const [sel, setSel] = useState<string[]>([])
  const [bulkDel, setBulkDel] = useState(false)
  const [monAll, setMonAll] = useState<{ s: Srv; d: any; err: string }[] | null>(null)
  const [mcfg, setMcfg] = useState<{ cpuPct: number; memPct: number; diskPct: number } | null>(null)
  const [monAllBusy, setMonAllBusy] = useState(false)

  useEffect(() => {
    api<any>('/api/tools/srv/moncfg')
      .then((c) => setMcfg({ cpuPct: c.cpuPct || 90, memPct: c.memPct || 90, diskPct: c.diskPct || 90 }))
      .catch(() => setMcfg({ cpuPct: 90, memPct: 90, diskPct: 90 }))
  }, [])

  const loadMon = async (s: Srv) => {
    setMonBusy(true)
    setMonErr('')
    setMonData(null)
    try {
      setMonData(await api('/api/tools/srv/mon?id=' + encodeURIComponent(s.id) + '&t=' + Date.now()))
    } catch (e: any) {
      setMonErr(e.message || 'وصل نشد')
    }
    setMonBusy(false)
  }

  const loadAllMon = async () => {
    setMonAllBusy(true)
    setMonAll([])
    const rows = await Promise.all(
      list.map(async (s) => {
        try {
          const d = await api('/api/tools/srv/mon?id=' + encodeURIComponent(s.id) + '&t=' + Date.now())
          return { s, d, err: '' }
        } catch (e: any) {
          return { s, d: null, err: e.message || 'وصل نشد' }
        }
      }),
    )
    setMonAll(rows)
    setMonAllBusy(false)
  }

  const runAct = async () => {
    if (!actSrv || !act) return
    try {
      await api(act === 'reboot' ? '/api/tools/srv/reboot' : '/api/tools/srv/upd', { id: actSrv.id })
      setNotice(act === 'reboot' ? 'دستور ریبوت ارسال شد.' : 'آپدیت شروع شد؛ نتیجه در تلگرام می‌آید.')
    } catch (e: any) {
      setNotice(e.message || 'انجام نشد')
    }
    setAct(null)
    setActSrv(null)
  }
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Skeleton />
  const list = data || []
  const needle = q.trim().toLowerCase()
  const shown = list
    .filter((s) => (flt === 'key' ? s.key : flt === 'pw' ? !s.key : true))
    .filter((s) => !needle || (s.name + ' ' + s.host + ' ' + s.user + ' ' + s.note).toLowerCase().includes(needle))
  const toggle = (id: string) => setSel((v) => (v.includes(id) ? v.filter((x) => x !== id) : [...v, id]))
  const bulkRun = async (kind: 'reboot' | 'upd') => {
    const ids = [...sel]
    setSel([])
    setNotice('در حال اجرا روی ' + faNum(ids.length) + ' سرور…')
    let ok = 0
    for (const id of ids) {
      try {
        await api(kind === 'reboot' ? '/api/tools/srv/reboot' : '/api/tools/srv/upd', { id })
        ok++
      } catch {
        /* one failure does not stop the rest */
      }
    }
    setNotice(kind === 'reboot' ? `ریبوت برای ${faNum(ok)} سرور ارسال شد.` : `آپدیت ${faNum(ok)} سرور شروع شد؛ نتیجه در تلگرام می‌آید.`)
  }
  const bulkRemove = async () => {
    const ids = [...sel]
    for (const id of ids) {
      try {
        await api('/api/tools/srv/del', { id })
      } catch {
        /* skip */
      }
    }
    setSel([])
    setBulkDel(false)
    invalidateCache()
    reload()
  }

  const submit = async () => {
    if (!edit) return
    setFormErr('')
    try {
      await save(edit.id ? '/api/tools/srv/edit' : '/api/tools/srv/add', edit, () => {
        setEdit(null)
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
          { label: 'سرورها', value: faNum(list.length) },
          { label: 'با کلید', value: faNum(list.filter((s) => s.key).length) },
          { label: 'با رمز', value: faNum(list.filter((s) => !s.key).length) },
        ]}
      />
      <div className="cc-pagebar">
        <Button variant="primary" icon={Plus} onClick={() => setEdit({ port: 22, user: 'root' })}>
          افزودن سرور
        </Button>
        <Button variant="outline" icon={Pulse} onClick={loadAllMon}>
          مانیتور همه
        </Button>
        <Button variant="outline" icon={ShieldCheck} onClick={() => setPwOpen(true)}>
          رمزها
        </Button>
        <Button variant="outline" icon={Gear} onClick={() => setCfgOpen(true)}>
          آستانه‌ها
        </Button>
        <Button variant="outline" icon={Plus} onClick={() => setImpOpen(true)}>
          افزودن گروهی
        </Button>
        <Button variant="outline" icon={Plus} onClick={() => setSubOpen(true)}>
          از ساب‌دامنه
        </Button>
      </div>
      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی سرورها"
        filters={[
          { id: 'all', label: 'همه سرورها' },
          { id: 'key', label: 'با کلید' },
          { id: 'pw', label: 'با رمز' },
        ]}
        value={flt}
        onValue={setFlt}
        onRefresh={() => {
          invalidateCache()
          reload()
        }}
      />
      {sel.length > 0 && (
        <div className="cc-bulkbar">
          <span>{faNum(sel.length)} انتخاب شده</span>
          <Button variant="outline" onClick={() => bulkRun('reboot')}>
            ریبوت
          </Button>
          <Button variant="outline" onClick={() => bulkRun('upd')}>
            آپدیت
          </Button>
          <Button variant="destructive" onClick={() => setBulkDel(true)}>
            حذف
          </Button>
        </div>
      )}
      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}
      {shown.length === 0 && (
        <Empty title="سروری نیست" description={needle ? 'چیزی با این جستجو پیدا نشد.' : 'با دکمهٔ «افزودن سرور» یکی اضافه کن.'} />
      )}
      {shown.map((s) => (
        <div
          key={s.id}
          className={'cc-trow cc-tap' + (sel.includes(s.id) ? ' sel' : '')}
          onClick={() => setDetail(s)}
        >
          <label className="cc-pick" onClick={(e) => e.stopPropagation()}>
            <input type="checkbox" id={'pick-' + s.id} name="pick" checked={sel.includes(s.id)} onChange={() => toggle(s.id)} aria-label="انتخاب سرور" />
          </label>
          <span className={s.key ? 'cc-dot' : 'cc-dot pw'} title={s.key ? 'کلید SSH' : 'رمز SSH'} />
          <div className="cc-grow">
            <div className="cc-name">{s.name || s.host}</div>
            <div className="cc-sub">
              {s.user}@{s.host}:{s.port}
            </div>
            {s.note && <div className="cc-sub">{s.note}</div>}
          </div>
          <Badge>{s.key ? 'کلید' : 'رمز'}</Badge>
        </div>
      ))}

      {detail && (
        <Dialog.Root open onOpenChange={(o) => !o && setDetail(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">{detail.name || detail.host}</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              {detail.user}@{detail.host}:{detail.port}
            </Dialog.Description>
            <div className="cc-actions">
              <Button
                variant="outline"
                icon={Pulse}
                onClick={() => {
                  const s = detail
                  setDetail(null)
                  setMon(s)
                  loadMon(s)
                }}
              >
                مانیتورینگ
              </Button>
              <Button
                variant="outline"
                icon={Info}
                onClick={() => {
                  const s = detail
                  setDetail(null)
                  setInfo({ s, kind: 'info' })
                }}
              >
                مشخصات سیستم
              </Button>
              <Button
                variant="outline"
                icon={HardDrive}
                onClick={() => {
                  const s = detail
                  setDetail(null)
                  setInfo({ s, kind: 'disk' })
                }}
              >
                فضای دیسک
              </Button>
              <Button
                variant="outline"
                icon={Gear}
                onClick={() => {
                  setDetail(null)
                  setCfgOpen(true)
                }}
              >
                آستانه‌های هشدار
              </Button>
              <Button
                variant="outline"
                icon={Copy}
                onClick={() => {
                  fallbackCopy(`ssh ${detail.user}@${detail.host} -p ${detail.port}`)
                  setNotice('دستور SSH کپی شد.')
                }}
              >
                کپی دستور SSH
              </Button>
              <Button
                variant="outline"
                icon={Power}
                onClick={() => {
                  const s = detail
                  setDetail(null)
                  setActSrv(s)
                  setAct('reboot')
                }}
              >
                ریبوت
              </Button>
              <Button
                variant="outline"
                icon={ArrowClockwise}
                onClick={() => {
                  const s = detail
                  setDetail(null)
                  setActSrv(s)
                  setAct('upd')
                }}
              >
                آپدیت
              </Button>
              <Button
                variant="outline"
                icon={PencilSimple}
                onClick={() => {
                  const s = detail
                  setDetail(null)
                  setEdit({ ...s, secret: '' })
                }}
              >
                ویرایش
              </Button>
              <Button
                variant="destructive"
                icon={Trash}
                onClick={() => {
                  const s = detail
                  setDetail(null)
                  setDel(s)
                }}
              >
                حذف
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {info && <TextDialog s={info.s} kind={info.kind} onClose={() => setInfo(null)} />}
      {cfgOpen && <MonCfgDialog onClose={() => setCfgOpen(false)} />}
      {impOpen && (
        <ImportDialog
          onClose={() => setImpOpen(false)}
          done={() => {
            setImpOpen(false)
            reload()
          }}
        />
      )}
      {pwOpen && <PwDialog onClose={() => setPwOpen(false)} />}
      {subOpen && (
        <SubDialog
          onClose={() => setSubOpen(false)}
          done={() => {
            setSubOpen(false)
            reload()
          }}
        />
      )}
      {monAll && (
        <Dialog.Root open onOpenChange={(o) => !o && setMonAll(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">مانیتور همه سرورها</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              {monAllBusy ? 'در حال بررسی…' : `${faNum(monAll.length)} سرور بررسی شد`}
            </Dialog.Description>
            {monAllBusy && (
              <div className="cc-center cc-mb">
                <Loader size="lg" />
              </div>
            )}
            {!monAllBusy &&
              monAll.map((r) => {
                const cpu = Math.round(Number(r.d?.cpu?.pct) || 0)
                const mem = Math.round(Number(r.d?.mem?.pct) || 0)
                const disk = Math.round(parseFloat(String(r.d?.disk?.pct ?? '')) || 0)
                const t = mcfg || { cpuPct: 90, memPct: 90, diskPct: 90 }
                const days = Math.floor((Number(r.d?.uptimeS) || 0) / 86400)
                return (
                  <div key={r.s.id} className="cc-trow">
                    <span className={r.err ? 'cc-dot pw' : cpu >= t.cpuPct || mem >= t.memPct || disk >= t.diskPct ? 'cc-dot hot' : 'cc-dot'} />
                    <div className="cc-grow">
                      <div className="cc-name">{r.s.name || r.s.host}</div>
                      <div className="cc-sub cc-ltr">
                        {r.s.user}@{r.s.host}:{r.s.port}
                      </div>
                      {r.err ? (
                        <div className="cc-sub cc-hot">{r.err}</div>
                      ) : (
                        <div className="cc-monrow">
                          <span className={cpu >= t.cpuPct ? 'cc-hot' : ''}>
                            CPU {cpu}%{r.d?.cpu?.cores ? ' · ' + r.d.cpu.cores + ' هسته' : ''}
                          </span>
                          <span className={mem >= t.memPct ? 'cc-hot' : ''}>
                            RAM {mem}% · {r.d?.mem?.usedMb ?? '—'}/{r.d?.mem?.totalMb ?? '—'} MB
                          </span>
                          <span className={disk >= t.diskPct ? 'cc-hot' : ''}>
                            دیسک {disk}% · {r.d?.disk?.used || '—'} / {r.d?.disk?.total || '—'}
                          </span>
                          <span>آپ‌تایم {days} روز</span>
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            <div className="cc-dialog-actions">
              <Button variant="secondary" icon={ArrowClockwise} onClick={loadAllMon}>
                تازه‌سازی
              </Button>
              <Button variant="primary" onClick={() => setMonAll(null)}>
                بستن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
      {mon && (
        <Dialog.Root open onOpenChange={(o) => !o && setMon(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">{mon.name || mon.host}</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              {mon.user}@{mon.host}:{mon.port}
            </Dialog.Description>
            {monBusy && (
              <div className="cc-center cc-mb">
                <Loader size="lg" />
              </div>
            )}
            {monErr && <div className="cc-form-err">{monErr}</div>}
            {monData && !monBusy && (
              <>
                <div className="cc-mon">
                  <div>
                    <span>CPU</span>
                    <b className={Number(monData.cpu?.pct) >= (mcfg?.cpuPct ?? 90) ? 'cc-hot' : ''}>
                      {Math.round(Number(monData.cpu?.pct) || 0)}%
                    </b>
                    <i>{monData.cpu?.cores ? monData.cpu.cores + ' هسته' : '—'}</i>
                  </div>
                  <div>
                    <span>RAM</span>
                    <b className={Number(monData.mem?.pct) >= (mcfg?.memPct ?? 90) ? 'cc-hot' : ''}>
                      {Math.round(Number(monData.mem?.pct) || 0)}%
                    </b>
                    <i>
                      {monData.mem?.usedMb ?? '—'} / {monData.mem?.totalMb ?? '—'} MB
                    </i>
                  </div>
                  <div>
                    <span>دیسک</span>
                    <b className={parseFloat(String(monData.disk?.pct ?? '')) >= (mcfg?.diskPct ?? 90) ? 'cc-hot' : ''}>
                      {monData.disk?.pct || '—'}
                    </b>
                    <i>
                      {monData.disk?.used || '—'} / {monData.disk?.total || '—'}
                    </i>
                  </div>
                </div>
                <div className="cc-hint cc-center">
                  آپ‌تایم: {Math.floor((Number(monData.uptimeS) || 0) / 86400)} روز
                </div>
              </>
            )}
            <div className="cc-dialog-actions">
              <Button variant="secondary" icon={ArrowClockwise} onClick={() => mon && loadMon(mon)}>
                تازه‌سازی
              </Button>
              <Button variant="outline" icon={Power} onClick={() => setAct('reboot')}>
                ریبوت
              </Button>
              <Button variant="primary" onClick={() => setAct('upd')}>
                آپدیت
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {act && actSrv && (
        <Dialog.Root open onOpenChange={(o) => !o && setAct(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">
              {act === 'reboot' ? 'ریبوت سرور' : 'آپدیت سرور'}
            </Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              {act === 'reboot'
                ? `«${actSrv.name || actSrv.host}» ریبوت شود؟ اتصال چند لحظه قطع می‌شود.`
                : `apt update && apt upgrade روی «${actSrv.name || actSrv.host}» اجرا شود؟`}
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setAct(null)}>
                انصراف
              </Button>
              <Button variant={act === 'reboot' ? 'destructive' : 'primary'} onClick={runAct}>
                {act === 'reboot' ? 'ریبوت کن' : 'اجرا کن'}
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {edit && (
        <FormDialog
          title={edit.id ? 'ویرایش سرور' : 'افزودن سرور'}
          sub={edit.id ? 'رمز را خالی بگذار تا تغییر نکند.' : undefined}
          onClose={() => setEdit(null)}
          onSave={submit}
          error={formErr}
        >
          <label className="cc-field">
            <span>نام</span>
            <Input value={edit.name || ''} onChange={(e) => setEdit({ ...edit, name: e.target.value })} placeholder="usa-1" />
          </label>
          <label className="cc-field">
            <span>هاست یا آی‌پی</span>
            <Input
              value={edit.host || ''}
              onChange={(e) => setEdit({ ...edit, host: e.target.value })}
              placeholder="1.2.3.4 یا domain.com"
              dir="ltr"
            />
          </label>
          <div className="cc-field-row">
            <label className="cc-field">
              <span>کاربر</span>
              <Input value={edit.user || ''} onChange={(e) => setEdit({ ...edit, user: e.target.value })} dir="ltr" />
            </label>
            <label className="cc-field cc-field-sm">
              <span>پورت</span>
              <Input
                type="number"
                value={String(edit.port ?? 22)}
                onChange={(e) => setEdit({ ...edit, port: Number(e.target.value) })}
                dir="ltr"
              />
            </label>
          </div>
          <label className="cc-field">
            <span>رمز یا کلید خصوصی</span>
            <textarea
              className="cc-input cc-textarea"
              rows={3}
              value={edit.secret || ''}
              onChange={(e) => setEdit({ ...edit, secret: e.target.value })}
              placeholder={edit.id ? 'برای تغییر نکردن خالی بگذار' : 'خالی = رمزهای ذخیره‌شده یکی‌یکی امتحان می‌شوند'}
              dir="ltr"
            />
          </label>
          {!edit.id && (
            <div className="cc-hint">
              اگر رمز را ندانی، خالی بگذار؛ سرور با رمزهای ذخیره‌شده تست می‌شود و اگر هیچ‌کدام جواب نداد، همین‌جا خطا می‌دهد.
            </div>
          )}
          <label className="cc-field">
            <span>یادداشت</span>
            <Input value={edit.note || ''} onChange={(e) => setEdit({ ...edit, note: e.target.value })} />
          </label>
        </FormDialog>
      )}

      {bulkDel && (
        <Dialog.Root open onOpenChange={(o) => !o && setBulkDel(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف گروهی</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              {faNum(sel.length)} سرور از لیست حذف شوند؟ فقط رکورد پنل پاک می‌شود، خود سرورها دست‌نخورده می‌مانند.
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setBulkDel(false)}>
                انصراف
              </Button>
              <Button variant="destructive" onClick={bulkRemove}>
                حذف کن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {del && (
        <Dialog.Root open onOpenChange={(o) => !o && setDel(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف سرور</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              «{del.name || del.host}» از لیست حذف شود؟
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setDel(null)}>
                انصراف
              </Button>
              <Button
                variant="destructive"
                onClick={async () => {
                  await save('/api/tools/srv/del', { id: del.id }, () => {
                    setDel(null)
                    reload()
                  })
                }}
              >
                حذف
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

    </>
  )
}

function RemView() {
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
  if (err) return <Empty title="خطا" description={err} />
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
      <div className="cc-pagebar">
        <Button variant="primary" icon={Plus} onClick={() => setAdd(true)}>
          یادآور جدید
        </Button>
      </div>
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

function PgView() {
  const { data, err, loaded, reload } = useTool<PgData>('/api/tools/pg')
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')
  const [pf, setPf] = useState<{ idx?: number; name: string; url: string; user: string; pass: string } | null>(null)
  const [pfErr, setPfErr] = useState('')
  const [pdel, setPdel] = useState<PgPanel | null>(null)
  const [mf, setMf] = useState<{ panel_url: string; name: string; interval: number } | null>(null)
  const [mfErr, setMfErr] = useState('')
  const [mdel, setMdel] = useState<PgMon | null>(null)
  const [nd, setNd] = useState<PgMon | null>(null)
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Skeleton />
  const iso0 = data || { panels: [], monitors: [] }
  const needle = q.trim().toLowerCase()
  const iso = needle
    ? {
        panels: iso0.panels.filter((p) => (p.name + ' ' + p.url + ' ' + p.user).toLowerCase().includes(needle)),
        monitors: iso0.monitors.filter((m) => (m.name + ' ' + m.panel).toLowerCase().includes(needle)),
      }
    : iso0
  const on = iso.monitors.filter((m) => m.enabled).length

  const submitPanel = async () => {
    if (!pf) return
    setPfErr('')
    try {
      await save(pf.idx === undefined ? '/api/tools/pg/panel/add' : '/api/tools/pg/panel/edit', pf, () => {
        setPf(null)
        reload()
      })
    } catch (e: any) {
      setPfErr(e.message || 'ذخیره نشد')
    }
  }
  const submitMon = async () => {
    if (!mf) return
    setMfErr('')
    try {
      await save('/api/tools/pg/mon/add', mf, () => {
        setMf(null)
        reload()
      })
    } catch (e: any) {
      setMfErr(e.message || 'ذخیره نشد')
    }
  }
  const run = async (path: string, body: any) => {
    try {
      await save(path, body, reload)
    } catch (e: any) {
      alert(e.message || 'انجام نشد')
    }
  }

  return (
    <>
      <StatGrid
        items={[
          { label: 'پنل‌ها', value: faNum(iso0.panels.length) },
          { label: 'مانیتور نود', value: faNum(iso0.monitors.length) },
          { label: 'فعال', value: faNum(on) },
        ]}
      />
      <div className="cc-pagebar">
        <Button variant="primary" icon={Plus} onClick={() => setPf({ name: '', url: '', user: '', pass: '' })}>
          افزودن پنل
        </Button>
      </div>
      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی پاسارگارد"
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'panels', label: 'پنل‌ها' },
          { id: 'mons', label: 'مانیتور نود' },
        ]}
        value={flt}
        onValue={setFlt}
        onRefresh={() => {
          invalidateCache()
          reload()
        }}
      />
      {flt !== 'mons' && <div className="cc-section">پنل‌ها</div>}
      {flt !== 'mons' && iso.panels.length === 0 && <Empty title="پنلی ثبت نشده" description="با دکمهٔ «افزودن پنل» یکی اضافه کن." />}
      {flt !== 'mons' && iso.panels.map((p) => (
        <div key={p.url} className="cc-trow cc-tap" onClick={() => setPf({ idx: p.idx, name: p.name, url: p.url, user: p.user, pass: '' })}>
          <div className="cc-grow">
            <div className="cc-name">{p.name || p.url}</div>
            <div className="cc-sub cc-mono">{p.url}</div>
          </div>
          {p.user && <Badge>{p.user}</Badge>}
          <div className="cc-rowbtns" onClick={(e) => e.stopPropagation()}>
            <button type="button" aria-label="حذف" className="danger" onClick={() => setPdel(p)}>
              <Trash size={17} />
            </button>
          </div>
        </div>
      ))}
      {flt !== 'panels' && <div className="cc-section">مانیتور نودها</div>}
      {flt !== 'panels' && <div className="cc-btnrow">
        <Button variant="outline" icon={Plus} onClick={() => setMf({ panel_url: iso0.panels[0]?.url || '', name: '', interval: 5 })}>
          افزودن مانیتور
        </Button>
      </div>}
      {flt !== 'panels' && iso.monitors.length === 0 && <Empty title="مانیتوری نیست" description="با دکمهٔ بالا یک مانیتور نود بساز." />}
      {flt !== 'panels' && iso.monitors.map((m) => (
        <div key={m.id} className="cc-trow cc-tap" onClick={() => setNd(m)}>
          <div className="cc-grow">
            <div className="cc-name">{m.name || m.id}</div>
            <div className="cc-sub">
              {m.panel} · هر {faNum(m.interval)} دقیقه
            </div>
          </div>
          <div className="cc-rowbtns" onClick={(e) => e.stopPropagation()}>
            <button
              type="button"
              aria-label={m.enabled ? 'خاموش کن' : 'روشن کن'}
              onClick={() => run('/api/tools/pg/mon/toggle', { id: m.id, enabled: !m.enabled })}
            >
              <Power size={17} />
            </button>
            <button type="button" aria-label="حذف" className="danger" onClick={() => setMdel(m)}>
              <Trash size={17} />
            </button>
          </div>
        </div>
      ))}

      {nd && <NdDialog m={nd} onClose={() => setNd(null)} onChanged={reload} />}

      {pf && (
        <FormDialog
          title={pf.idx === undefined ? 'افزودن پنل' : 'ویرایش پنل'}
          sub={pf.idx === undefined ? 'ورود قبل از ذخیره تست می‌شود.' : 'رمز را خالی بگذار تا تغییر نکند.'}
          onClose={() => setPf(null)}
          onSave={submitPanel}
          error={pfErr}
        >
          <label className="cc-field">
            <span>نام</span>
            <Input value={pf.name} onChange={(e) => setPf({ ...pf, name: e.target.value })} placeholder="main" />
          </label>
          <label className="cc-field">
            <span>آدرس پنل</span>
            <Input value={pf.url} onChange={(e) => setPf({ ...pf, url: e.target.value })} placeholder="https://panel.example.com" />
          </label>
          <label className="cc-field">
            <span>نام کاربری</span>
            <Input value={pf.user} onChange={(e) => setPf({ ...pf, user: e.target.value })} placeholder="admin" />
          </label>
          <label className="cc-field">
            <span>رمز</span>
            <Input type="password" value={pf.pass} onChange={(e) => setPf({ ...pf, pass: e.target.value })} placeholder="••••••" />
          </label>
        </FormDialog>
      )}

      {mf && (
        <FormDialog title="افزودن مانیتور نود" onClose={() => setMf(null)} onSave={submitMon} error={mfErr}>
          <label className="cc-field">
            <span>پنل</span>
            <select className="cc-input" value={mf.panel_url} onChange={(e) => setMf({ ...mf, panel_url: e.target.value })}>
              {iso0.panels.map((p) => (
                <option key={p.url} value={p.url}>
                  {p.name || p.url}
                </option>
              ))}
            </select>
          </label>
          <label className="cc-field">
            <span>نام نمایشی</span>
            <Input value={mf.name} onChange={(e) => setMf({ ...mf, name: e.target.value })} placeholder="node-1" />
          </label>
          <label className="cc-field">
            <span>فاصلهٔ بررسی (دقیقه)</span>
            <Input
              type="number"
              value={String(mf.interval)}
              onChange={(e) => setMf({ ...mf, interval: Number(e.target.value) || 5 })}
            />
          </label>
        </FormDialog>
      )}

      {pdel && (
        <Dialog.Root open onOpenChange={(o) => !o && setPdel(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف پنل</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              «{pdel.name || pdel.url}» و مانیتورهای نودش حذف می‌شوند. روی پاسارگارد اثری ندارد.
            </Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setPdel(null)}>
                انصراف
              </Button>
              <Button variant="destructive" onClick={() => pdel && run('/api/tools/pg/panel/del', { idx: pdel.idx })}>
                حذف کن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {mdel && (
        <Dialog.Root open onOpenChange={(o) => !o && setMdel(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">حذف مانیتور</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">مانیتور «{mdel.name || mdel.id}» حذف شود؟</Dialog.Description>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setMdel(null)}>
                انصراف
              </Button>
              <Button variant="destructive" onClick={() => mdel && run('/api/tools/pg/mon/del', { id: mdel.id })}>
                حذف کن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
    </>
  )
}


// ---------------- node monitor detail ----------------
type NdNode = { name: string; dir: string; addr?: string; version?: string; reason?: string; ts?: string; exc: boolean }
type NdData = { id: string; name: string; panel: string; interval: number; enabled: boolean; hook: string; nodes: NdNode[] }

function NdDialog({ m, onClose, onChanged }: { m: PgMon; onClose: () => void; onChanged: () => void }) {
  const [st, setSt] = useState<NdData | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState('')
  const [interval, setInterval] = useState(m.interval || 10)
  const [notice, setNotice] = useState('')

  const load = async () => {
    setErr('')
    try {
      setSt(await api<NdData>('/api/tools/pg/mon?id=' + encodeURIComponent(m.id) + '&t=' + Date.now()))
    } catch (e: any) {
      setErr(e.message || 'خوانده نشد')
    }
  }
  useEffect(() => {
    load()
  }, [m.id])

  const post = async (path: string, body: any, label: string) => {
    setBusy(label)
    setErr('')
    setNotice('')
    try {
      const res = await api<NdData>(path, body)
      if (res && res.nodes) setSt(res)
      invalidateCache()
      onChanged()
    } catch (e: any) {
      setErr(e.message || 'انجام نشد')
    } finally {
      setBusy('')
    }
  }

  const nodes = st?.nodes || []
  const up = nodes.filter((n) => n.dir === 'up').length
  const down = nodes.filter((n) => n.dir === 'down').length

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">{m.name || m.id}</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc cc-mono">{m.panel}</Dialog.Description>

        {err && <div className="cc-form-err cc-mb">{err}</div>}
        {notice && <div className="cc-hint cc-mb">{notice}</div>}
        {!st && !err && <Loader size="base" />}

        {st && (
          <div className="cc-moncc">
            <div className="cc-mon">
              <div>
                <span>نودها</span>
                <b>{faNum(nodes.length)}</b>
              </div>
              <div>
                <span>سالم</span>
                <b>{faNum(up)}</b>
              </div>
              <div>
                <span>قطع</span>
                <b>{faNum(down)}</b>
              </div>
            </div>

            <div className="cc-field-row">
              <label className="cc-field cc-field-sm">
                <span>فاصله (دقیقه)</span>
                <Input
                  type="number"
                  value={String(interval)}
                  onChange={(e) => setInterval(Number(e.target.value) || 1)}
                />
              </label>
              <Button
                variant="outline"
                disabled={!!busy}
                onClick={() => post('/api/tools/pg/mon/cfg', { id: m.id, interval, enabled: st.enabled }, 'cfg')}
              >
                ذخیرهٔ فاصله
              </Button>
            </div>

            <div className="cc-btnrow cc-gridbtns">
              <Button variant="outline" icon={ArrowClockwise} disabled={!!busy} onClick={() => post('/api/tools/pg/mon/sync', { id: m.id }, 'sync')}>
                همگام‌سازی
              </Button>
              <Button variant="outline" icon={BellRinging} disabled={!!busy} onClick={() => post('/api/tools/pg/mon/test', { id: m.id }, 'test').then(() => setNotice('پیام تستی در تلگرام ارسال شد.'))}>
                تست هشدار
              </Button>
              <Button variant="outline" icon={Power} disabled={!!busy} onClick={() => post('/api/tools/pg/mon/cfg', { id: m.id, interval, enabled: !st.enabled }, 'tg')}>
                {st.enabled ? 'توقف پایش' : 'فعال‌سازی'}
              </Button>
              <Button
                variant="outline"
                icon={Copy}
                onClick={() => {
                  fallbackCopy(st.hook)
                  setNotice('آدرس وب‌هوک کپی شد.')
                }}
              >
                کپی وب‌هوک
              </Button>
            </div>
            <div className="cc-hookurl">{st.hook}</div>

            {nodes.length === 0 && <Empty title="نودی ثبت نشده" description="«همگام‌سازی» را بزن تا نودها از پنل خوانده شوند." />}
            {nodes.map((n) => (
              <div key={n.name} className="cc-trow">
                <div className="cc-grow">
                  <div className="cc-name">{n.name}</div>
                  <div className="cc-sub">
                    {[n.addr, n.version, n.reason].filter(Boolean).join(' · ') || '—'}
                  </div>
                </div>
                <Badge variant={n.dir === 'up' ? 'success' : n.dir === 'down' ? 'error' : 'neutral'}>
                  {n.dir === 'up' ? 'سالم' : n.dir === 'down' ? 'قطع' : 'نامشخص'}
                </Badge>
                <button
                  type="button"
                  className={n.exc ? 'cc-rowbtn' : 'cc-rowbtn danger'}
                  title={n.exc ? 'حذف از استثنا' : 'استثنا کردن این نود'}
                  disabled={!!busy}
                  onClick={() => post('/api/tools/pg/mon/exc', { id: m.id, node: n.name }, 'exc')}
                >
                  <BellSimple size={16} />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="cc-dialog-actions">
          <Button variant="secondary" onClick={onClose}>
            بستن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

/* ---------------- server extras ---------------- */

function TextDialog({ s, kind, onClose }: { s: Srv; kind: 'info' | 'disk'; onClose: () => void }) {
  const [text, setText] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(true)
  const load = () => {
    setBusy(true)
    setErr('')
    api<{ text: string }>(`/api/tools/srv/${kind}?id=${encodeURIComponent(s.id)}&t=${Date.now()}`)
      .then((d) => setText(d.text))
      .catch((e) => setErr(e.message || 'خطا'))
      .finally(() => setBusy(false))
  }
  useEffect(load, [])
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">{kind === 'info' ? 'مشخصات سیستم' : 'فضای دیسک'}</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc">{s.name || s.host}</Dialog.Description>
        {busy && (
          <div className="cc-center cc-mb">
            <Loader size="lg" />
          </div>
        )}
        {err && <ErrBox msg={err} />}
        {!busy && !err && <pre className="cc-pre">{text || '—'}</pre>}
        <div className="cc-dialog-actions">
          <Button variant="secondary" icon={ArrowClockwise} onClick={load}>
            تازه‌سازی
          </Button>
          <Button variant="primary" onClick={onClose}>
            بستن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

function MonCfgDialog({ onClose }: { onClose: () => void }) {
  const [cfg, setCfg] = useState<any>(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    api<any>('/api/tools/srv/moncfg')
      .then(setCfg)
      .catch((e) => setErr(e.message || 'خطا'))
  }, [])
  const submit = async () => {
    setErr('')
    try {
      await save('/api/tools/srv/moncfg', cfg, onClose)
    } catch (e: any) {
      setErr(e.message || 'ذخیره نشد')
    }
  }
  const num = (k: string, label: string, hint: string) => (
    <label className="cc-field">
      <span>{label}</span>
      <Input
        type="number"
        value={String(cfg?.[k] ?? '')}
        onChange={(e) => setCfg({ ...cfg, [k]: Number(e.target.value) })}
        placeholder={hint}
      />
    </label>
  )
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">آستانه‌های هشدار</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc">وقتی از این عددها بگذرد در تلگرام خبر می‌دهد.</Dialog.Description>
        {!cfg && !err && (
          <div className="cc-center cc-mb">
            <Loader size="lg" />
          </div>
        )}
        {err && <ErrBox msg={err} />}
        {cfg && (
          <div className="cc-form">
            <label className="cc-checkline">
              <input
                type="checkbox"
                checked={!!cfg.enabled}
                onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })}
              />
              <span>پایش فعال باشد</span>
            </label>
            {num('cpuPct', 'CPU (٪)', '90')}
            {num('memPct', 'RAM (٪)', '90')}
            {num('diskPct', 'دیسک (٪)', '90')}
            {num('cooldownMin', 'فاصلهٔ هشدار (دقیقه)', '60')}
          </div>
        )}
        <div className="cc-dialog-actions">
          <Button variant="secondary" onClick={onClose}>
            انصراف
          </Button>
          <Button variant="primary" onClick={submit} disabled={!cfg}>
            ذخیره
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

function ImportDialog({ onClose, done }: { onClose: () => void; done: () => void }) {
  const [lines, setLines] = useState('')
  const [err, setErr] = useState('')
  const [msg, setMsg] = useState('')
  const submit = async () => {
    setErr('')
    try {
      const r = await api<{ added: number; skipped: number }>('/api/tools/srv/import', { lines })
      setMsg(`${faNum(r.added)} سرور اضافه شد.`)
      invalidateCache()
      setTimeout(done, 600)
    } catch (e: any) {
      setErr(e.message || 'اضافه نشد')
    }
  }
  return (
    <FormDialog title="افزودن گروهی" sub="هر خط: هاست کاربر رمز [پورت]" onClose={onClose} onSave={submit} error={err}>
      <label className="cc-field">
        <span>لیست سرورها</span>
        <textarea
          className="cc-input cc-textarea"
          rows={6}
          value={lines}
          onChange={(e) => setLines(e.target.value)}
          placeholder={'1.2.3.4 root MyPass\n5.6.7.8 root - 2222'}
        />
      </label>
      <div className="cc-hint">رمز `-` یعنی بدون رمز. خطی که با # شروع شود نادیده گرفته می‌شود.</div>
      {msg && <div className="cc-hint">{msg}</div>}
    </FormDialog>
  )
}

function PwDialog({ onClose }: { onClose: () => void }) {
  const [list, setList] = useState<string[] | null>(null)
  const [val, setVal] = useState('')
  const [err, setErr] = useState('')
  const load = () =>
    api<{ masked: string[] }>('/api/tools/srv/pw')
      .then((d) => setList(d.masked || []))
      .catch((e) => setErr(e.message || 'خطا'))
  useEffect(() => {
    load()
  }, [])
  const add = async () => {
    if (!val.trim()) return
    try {
      await save('/api/tools/srv/pw', { secret: val }, () => {
        setVal('')
        load()
      })
    } catch (e: any) {
      setErr(e.message || 'ذخیره نشد')
    }
  }
  const del = async (i: number) => {
    try {
      await save('/api/tools/srv/pw', { del: i }, load)
    } catch (e: any) {
      setErr(e.message || 'حذف نشد')
    }
  }
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">رمزهای ذخیره‌شده</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc">در افزودن گروهی و از ساب‌دامنه قابل انتخاب‌اند.</Dialog.Description>
        {err && <ErrBox msg={err} />}
        {!list && (
          <div className="cc-center cc-mb">
            <Loader size="lg" />
          </div>
        )}
        {list && list.length === 0 && <div className="cc-hint cc-center cc-mb">رمزی ذخیره نشده.</div>}
        {list?.map((m, i) => (
          <div key={i} className="cc-trow">
            <div className="cc-avatar">{i + 1}</div>
            <div className="cc-grow">
              <div className="cc-name">{m}</div>
            </div>
            <div className="cc-rowbtns">
              <button type="button" className="danger" aria-label="حذف" onClick={() => del(i)}>
                <Trash size={17} />
              </button>
            </div>
          </div>
        ))}
        <div className="cc-field">
          <span>رمز جدید</span>
          <Input value={val} onChange={(e) => setVal(e.target.value)} placeholder="رمز" type="password" />
        </div>
        <div className="cc-dialog-actions">
          <Button variant="secondary" onClick={onClose}>
            بستن
          </Button>
          <Button variant="primary" icon={Plus} onClick={add}>
            افزودن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

type Fav = { zone: string; label: string; name?: string; content?: string; type?: string; missing?: boolean }

type Zone = { acc: number; id: string; name: string }
type Rec = { type: string; name: string; content: string }

function SubDialog({ onClose, done }: { onClose: () => void; done: () => void }) {
  const [zones, setZones] = useState<Zone[] | null>(null)
  const [zone, setZone] = useState<Zone | null>(null)
  const [recs, setRecs] = useState<Rec[] | null>(null)
  const [picked, setPicked] = useState<Record<string, boolean>>({})
  const [pws, setPws] = useState<string[]>([])
  const [pwIdx, setPwIdx] = useState<number | null>(null)
  const [custom, setCustom] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [src, setSrc] = useState<'zones' | 'favs'>('zones')
  const [favs, setFavs] = useState<Fav[] | null>(null)
  const [favPick, setFavPick] = useState<Record<string, boolean>>({})

  const loadFavs = () => {
    if (favs) return
    api<any>('/api/cf/favs')
      .then((d) => setFavs((d.items || []).filter((f: Fav) => !f.missing && f.content)))
      .catch((e) => setErr(e.message || 'خطا'))
  }

  useEffect(() => {
    api<any>('/api/cf/zones')
      .then((d) => setZones(d.zones || []))
      .catch((e) => setErr(e.message || 'خطا'))
    api<{ masked: string[] }>('/api/tools/srv/pw')
      .then((d) => setPws(d.masked || []))
      .catch(() => {})
  }, [])

  const pickZone = (z: Zone) => {
    setZone(z)
    setRecs(null)
    setPicked({})
    api<any>(`/api/cf/records?acc=${z.acc}&zid=${encodeURIComponent(z.id)}`)
      .then((d) => setRecs((d.records || []).filter((r: Rec) => r.type === 'A' || r.type === 'AAAA')))
      .catch((e) => setErr(e.message || 'خطا'))
  }

  const submit = async () => {
    if (src === 'favs') {
      const items = (favs || [])
        .filter((f, i) => favPick[`${f.zone}|${f.content}|${i}`])
        .map((f) => ({
          name: !f.name || f.name === '@' ? f.zone : `${f.name}.${f.zone}`,
          host: f.content as string,
        }))
      if (items.length === 0) {
        setErr('هیچ موردی انتخاب نشده')
        return
      }
      setBusy(true)
      setErr('')
      try {
        await api('/api/tools/srv/subadd', { items, password: pwIdx === null ? custom : '', pwIndex: pwIdx })
        done()
        onClose()
      } catch (e: any) {
        setErr(e.message || 'اضافه نشد')
      }
      setBusy(false)
      return
    }
    if (!zone || !recs) return
    const items = recs
      .map((r, i) => ({ r, key: `${r.type}|${r.name}|${r.content}|${i}` }))
      .filter((x) => picked[x.key])
      .map(({ r }) => ({ name: r.name === '@' ? zone.name : `${r.name}.${zone.name}`, host: r.content }))
    if (items.length === 0) {
      setErr('هیچ رکوردی انتخاب نشده')
      return
    }
    setBusy(true)
    setErr('')
    try {
      await api('/api/tools/srv/subadd', {
        items,
        password: pwIdx === null ? custom : '',
        pwIndex: pwIdx,
      })
      invalidateCache()
      setBusy(false)
      done()
    } catch (e: any) {
      setErr(e.message || 'اضافه نشد')
      setBusy(false)
    }
  }

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">از ساب‌دامنه</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc">دامنه را انتخاب کن و رکوردهای A/AAAA را تیک بزن.</Dialog.Description>
        {err && <ErrBox msg={err} />}
        {!zones && (
          <div className="cc-center cc-mb">
            <Loader size="lg" />
          </div>
        )}
        <div className="cc-btnrow cc-mb">
          <Button variant={src === 'zones' ? 'primary' : 'outline'} onClick={() => setSrc('zones')}>
            از زون‌ها
          </Button>
          <Button
            variant={src === 'favs' ? 'primary' : 'outline'}
            onClick={() => {
              setSrc('favs')
              loadFavs()
            }}
          >
            از موردعلاقه‌ها
          </Button>
        </div>
        {src === 'zones' && zones && !zone && (
          <div className="cc-form">
            {zones.length === 0 && <div className="cc-hint">زونی در کلادفلر نیست.</div>}
            {zones.map((z) => (
              <button key={z.id} type="button" className="cc-choice" onClick={() => pickZone(z)}>
                <span className="cc-choice-label">{z.name}</span>
              </button>
            ))}
          </div>
        )}
        {src === 'zones' && zone && (
          <>
            <div className="cc-hint cc-mb">{zone.name}</div>
            {!recs && (
              <div className="cc-center cc-mb">
                <Loader size="lg" />
              </div>
            )}
            {recs && recs.length === 0 && <div className="cc-hint cc-mb">رکورد A/AAAA ندارد.</div>}
            {recs?.map((r, i) => {
              const key = `${r.type}|${r.name}|${r.content}|${i}`
              return (
                <label key={key} className="cc-checkline">
                  <input
                    type="checkbox"
                    checked={!!picked[key]}
                    onChange={(e) => setPicked({ ...picked, [key]: e.target.checked })}
                  />
                  <span>
                    {r.name} · {r.type} · {r.content}
                  </span>
                </label>
              )
            })}
          </>
        )}
        {src === 'favs' && (
          <>
            {!favs && (
              <div className="cc-center cc-mb">
                <Loader size="lg" />
              </div>
            )}
            {favs?.length === 0 && <div className="cc-hint cc-mb">موردعلاقه‌ای با IP نیست.</div>}
            {favs?.map((f, i) => {
              const key = `${f.zone}|${f.content}|${i}`
              return (
                <label key={key} className="cc-checkline">
                  <input
                    type="checkbox"
                    checked={!!favPick[key]}
                    onChange={(e) => setFavPick({ ...favPick, [key]: e.target.checked })}
                  />
                  <span>
                    {f.label} · {f.content}
                  </span>
                </label>
              )
            })}
          </>
        )}
        {(src === 'favs' || zone) && (
          <>
            <div className="cc-field">
              <span>رمز پیش‌فرض</span>
              <select
                className="cc-input"
                value={pwIdx === null ? 'c' : String(pwIdx)}
                onChange={(e) => setPwIdx(e.target.value === 'c' ? null : Number(e.target.value))}
              >
                {pws.map((m, i) => (
                  <option key={i} value={i}>
                    ذخیره‌شده #{i + 1} ({m})
                  </option>
                ))}
                <option value="c">رمز دلخواه</option>
              </select>
            </div>
            {pwIdx === null && (
              <label className="cc-field">
                <span>رمز</span>
                <Input value={custom} onChange={(e) => setCustom(e.target.value)} placeholder="خالی = بدون رمز" />
              </label>
            )}
          </>
        )}
        <div className="cc-dialog-actions">
          <Button variant="secondary" onClick={onClose}>
            انصراف
          </Button>
          <Button variant="primary" onClick={submit} disabled={(src === 'zones' && !zone) || busy}>
            افزودن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

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
  useEffect(() => {
    api<UmCfg>('/api/tools/um')
      .then(setCfg)
      .catch((e) => setErr(e.message || 'خطا'))
  }, [])
  if (err) return <Empty title="خطا" description={err} />
  if (!cfg)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
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

type DomSsl = { host: string; ok: boolean; days: number; expiry: string; error: string }
type DomItem = { domain: string; status: string; expiry: number | null; manual: boolean; remaining: string; message?: string }
type DomData = { ssl: DomSsl[]; doms: DomItem[]; key: boolean; sslOn: boolean; warnDays: number }

const faTxt = (v: string) => (v || '').replace(/[0-9]/g, (d) => '۰۱۲۳۴۵۶۷۸۹'[Number(d)])

const DOM_STATUS: Record<string, string> = {
  ok: '',
  hidden: 'تاریخ توسط ایرنیک مخفی شده',
  no_date: 'تاریخ انقضا پیدا نشد',
  not_found: 'دامنه یافت نشد',
  available: 'دامنه آزاد است',
  api_error: 'خطا در استعلام از سرویس',
  invalid: 'نام دامنه نامعتبر',
  error: 'استعلام انجام نشد',
}

type DomCfg = { provider: string; hasKey: boolean; keyHint: string; providers: { id: string; label: string }[] }

// Provider + API key for the domain-expiry lookup, with a live test.
function DomCfgDialog({ onClose }: { onClose: () => void }) {
  const [cfg, setCfg] = useState<DomCfg | null>(null)
  const [provider, setProvider] = useState('')
  const [key, setKey] = useState('')
  const [test, setTest] = useState<{ status: string; message?: string; remaining?: string; domain?: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  useEffect(() => {
    api<DomCfg>('/api/tools/dom/cfg').then((d) => {
      setCfg(d)
      setProvider(d.provider)
    }).catch((e) => setErr(String(e)))
  }, [])

  async function runTest() {
    setBusy(true)
    setErr('')
    try {
      const r = await api<{ status: string; message?: string; remaining?: string; domain?: string }>(
        '/api/tools/dom/cfg/test',
        { domain: '' },
      )
      setTest(r)
    } catch (e) {
      setErr(String(e))
    }
    setBusy(false)
  }

  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">تنظیمات استعلام دامنه</Dialog.Title>
        <Dialog.Description className="cc-dialog-desc">
          بدون کلید، فقط سرویس‌های آزاد (RDAP/whois) استفاده می‌شود؛ بعضی دامنه‌ها تاریخ را مخفی می‌کنند.
        </Dialog.Description>
        <div className="cc-form">
          <label className="cc-field">
            <span>روش استعلام</span>
            <select className="cc-input" value={provider} onChange={(e) => setProvider(e.target.value)}>
              {(cfg?.providers || []).map((p) => (
                <option key={p.id} value={p.id}>{p.label}</option>
              ))}
            </select>
          </label>
          <label className="cc-field">
            <span>کلید API {cfg?.hasKey ? `(ذخیره‌شده: ${cfg.keyHint})` : '(اختیاری)'}</span>
            <Input className="cc-input" value={key} onChange={(e) => setKey(e.target.value)} placeholder="برای تغییر نکردن خالی بگذار" autoComplete="off" />
          </label>
          {test && (
            <div className="cc-hint">
              {test.domain}: {test.status}
              {test.remaining ? ` — ${test.remaining}` : ''}
              {test.message ? ` (${test.message})` : ''}
            </div>
          )}
          {err && <ErrBox msg={err} />}
        </div>
        <div className="cc-dialog-actions">
          <Button variant="outline" onClick={onClose}>بستن</Button>
          <Button variant="outline" onClick={runTest} disabled={busy}>{busy ? 'در حال تست…' : 'تست'}</Button>
          <Button
            variant="primary"
            onClick={async () => {
              setBusy(true)
              try {
                await api('/api/tools/dom/cfg', { provider, key })
                invalidateCache()
                onClose()
              } catch (e) {
                setErr(String(e))
              }
              setBusy(false)
            }}
            disabled={busy}
          >
            ذخیره
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

function DomView() {
  const { data, err, loaded, reload } = useTool<DomData>('/api/tools/dom')
  const [q, setQ] = useState('')
  const [flt, setFlt] = useState('all')
  const [addSsl, setAddSsl] = useState(false)
  const [host, setHost] = useState('')
  const [addDom, setAddDom] = useState(false)
  const [dom, setDom] = useState('')
  const [dateFor, setDateFor] = useState<DomItem | null>(null)
  const [when, setWhen] = useState('')
  const [fErr, setFErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [warnOn, setWarnOn] = useState<boolean | null>(null)
  const [warnDays, setWarnDays] = useState<number | null>(null)
  const [cfgOpen, setCfgOpen] = useState(false)

  if (err) return <Empty title="خطا" description={err} />
  if (!loaded || !data)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
    )

  const needle = q.trim().toLowerCase()
  const ssl0 = needle ? data.ssl.filter((s) => s.host.includes(needle)) : data.ssl
  const ssl = flt === 'near' ? ssl0.filter((s) => !s.ok || s.days <= (warnDays ?? data.warnDays)) : ssl0
  const doms = needle ? data.doms.filter((d) => d.domain.includes(needle)) : data.doms
  const showSsl = flt !== 'dom'
  const showDom = flt !== 'ssl' && flt !== 'near'
  const warn = warnDays ?? data.warnDays
  const on = warnOn ?? data.sslOn
  const bad = data.ssl.filter((s) => !s.ok || s.days <= warn).length

  const saveCfg = async () => {
    setBusy(true)
    setFErr('')
    try {
      await api('/api/tools/dom/sslcfg', { enabled: on, warnDays: warn })
      invalidateCache()
      setWarnOn(null)
      setWarnDays(null)
      reload()
    } catch (e: any) {
      setFErr(e.message || 'ذخیره نشد')
    } finally {
      setBusy(false)
    }
  }

  const act = async (path: string, body: any, done?: () => void) => {
    setFErr('')
    try {
      await api(path, body)
      invalidateCache()
      done?.()
      reload()
    } catch (e: any) {
      setFErr(e.message || 'انجام نشد')
    }
  }

  const refresh = async () => {
    setBusy(true)
    setFErr('')
    try {
      await api('/api/tools/dom/refresh', {})
      invalidateCache()
      reload()
    } catch (e: any) {
      setFErr(e.message || 'استعلام نشد')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <StatGrid
        items={[
          { label: 'گواهی SSL', value: faNum(data.ssl.length) },
          { label: 'نیاز به بررسی', value: faNum(bad) },
          { label: 'دامنه‌ها', value: faNum(data.doms.length) },
        ]}
      />
      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="جستجوی دامنه"
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'ssl', label: 'گواهی‌ها' },
          { id: 'near', label: 'نزدیک انقضا' },
          { id: 'dom', label: 'انقضای دامنه' },
        ]}
        value={flt}
        onValue={setFlt}
        menu={[{ id: 'cfg', label: 'تنظیمات استعلام' }]}
        onMenu={(id) => id === 'cfg' && setCfgOpen(true)}
        onRefresh={refresh}
        busy={busy}
      />
      {fErr && <div className="cc-form-err cc-mb">{fErr}</div>}

      {showSsl && <div className="cc-section">
        <div className="cc-section-head">
          <h2>گواهی SSL</h2>
          <Button variant="outline" icon={Plus} onClick={() => setAddSsl(true)}>
            افزودن دامنه
          </Button>
        </div>
        <div className="cc-checkline">
          <input
            type="checkbox"
            id="ssl-alert-on"
            name="sslAlertOn"
            aria-label="هشدار تلگرام گواهی"
            checked={on}
            onChange={(e) => setWarnOn(e.target.checked)}
          />
          <span>هشدار تلگرام</span>
          <span className="cc-grow" />
          <input
            className="cc-input cc-num"
            id="ssl-warn-days"
            name="sslWarnDays"
            aria-label="روزهای هشدار گواهی"
            type="number"
            min={1}
            max={365}
            value={warn}
            onChange={(e) => setWarnDays(Number(e.target.value))}
          />
          <span>روز مانده</span>
          <Button variant="outline" onClick={saveCfg} disabled={busy}>
            ذخیره
          </Button>
        </div>
        <div className="cc-hint cc-mb">وقتی گواهی به این تعداد روز یا کمتر برسد، روزی یک بار در تلگرام هشدار می‌آید.</div>
        {ssl.length === 0 && <Empty title="خالی است" description="دامنه‌ای برای نظارت گواهی ثبت نشده." />}
        {ssl.map((s) => (
          <div key={s.host} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">{s.host}</div>
              <div className="cc-sub">{s.ok ? 'انقضا: ' + faDate(s.expiry) : s.error}</div>
            </div>
            <Badge variant={s.ok && s.days > warn ? 'success' : 'error'}>
              {s.ok ? faNum(s.days) + ' روز' : 'خطا'}
            </Badge>
            <button className="cc-rowbtn danger" title="حذف" onClick={() => act('/api/tools/dom/ssl', { host: s.host })}>
              <Trash size={16} />
            </button>
          </div>
        ))}
      </div>}

      {showDom && <div className="cc-section">
        <div className="cc-section-head">
          <h2>انقضای دامنه</h2>
          <div className="cc-btnrow">
            <Button variant="outline" icon={ArrowClockwise} onClick={refresh} disabled={busy}>
              استعلام دوباره
            </Button>
            <Button variant="outline" icon={Plus} onClick={() => setAddDom(true)}>
              افزودن
            </Button>
          </div>
        </div>
        {!data.key && (
          <div className="cc-hint cc-mb">
            برای دامنه‌های .ir که ایرنیک تاریخ را مخفی کرده، تاریخ را دستی ثبت کن.
          </div>
        )}
        {doms.length === 0 && <Empty title="خالی است" description="دامنه‌ای برای نظارت انقضا ثبت نشده." />}
        {doms.map((d) => (
          <div key={d.domain} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">{d.domain}</div>
              <div className="cc-sub">{d.expiry ? 'انقضا: ' + faDate(d.expiry) : DOM_STATUS[d.status] || 'تاریخ نامشخص'}</div>
              {!d.expiry && d.message && <div className="cc-sub">{d.message}</div>}
            </div>
            <Badge>{faTxt(d.remaining) || DOM_STATUS[d.status] || d.status}</Badge>
            <button
              className="cc-rowbtn"
              title="استعلام دوباره"
              onClick={() => act('/api/tools/dom/one', { domain: d.domain })}
            >
              <ArrowClockwise size={16} />
            </button>
            <button
              className="cc-rowbtn"
              title="ثبت تاریخ"
              onClick={() => {
                setFErr('')
                setWhen('')
                setDateFor(d)
              }}
            >
              <PencilSimple size={16} />
            </button>
            <button className="cc-rowbtn danger" title="حذف" onClick={() => act('/api/tools/dom/del', { domain: d.domain })}>
              <Trash size={16} />
            </button>
          </div>
        ))}
      </div>}

      {addSsl && (
        <FormDialog
          title="افزودن گواهی SSL"
          sub="روزهای باقی‌مانده گواهی هر ۶ ساعت بررسی می‌شود."
          onClose={() => setAddSsl(false)}
          onSave={() => act('/api/tools/dom/ssl', { host }, () => { setAddSsl(false); setHost('') })}
          error={fErr}
        >
          <label className="cc-field">
            <span>دامنه</span>
            <Input value={host} onChange={(e) => setHost(e.target.value)} placeholder="example.com" />
          </label>
        </FormDialog>
      )}

      {addDom && (
        <FormDialog
          title="افزودن دامنه"
          sub="انقضا از RDAP و whois.nic.ir استعلام می‌شود."
          onClose={() => setAddDom(false)}
          onSave={() => act('/api/tools/dom/add', { domain: dom }, () => { setAddDom(false); setDom('') })}
          error={fErr}
        >
          <label className="cc-field">
            <span>دامنه</span>
            <Input value={dom} onChange={(e) => setDom(e.target.value)} placeholder="example.ir" />
          </label>
        </FormDialog>
      )}

      {dateFor && (
        <FormDialog
          title="ثبت تاریخ انقضا"
          sub={dateFor.domain}
          onClose={() => setDateFor(null)}
          onSave={() => act('/api/tools/dom/date', { domain: dateFor.domain, when }, () => setDateFor(null))}
          error={fErr}
        >
          <label className="cc-field">
            <span>تاریخ شمسی</span>
            <Input value={when} onChange={(e) => setWhen(e.target.value)} placeholder="1405/06/20" />
          </label>
        </FormDialog>
      )}

      {cfgOpen && <DomCfgDialog onClose={() => setCfgOpen(false)} />}
    </>
  )
}

// ---------------- host filter (auto sub swap) ----------------
type HfCfg = {
  enabled: boolean; intervalMin: number; cities: number; citiesSel: string[]; maxOk: number
  recheckCount: number; recheckMin: number; batch: number; maxChanges: number
  realityRotate: boolean; lastRun: string; lastSummary: string; excCount: number
}
type HfData = { cfg: HfCfg; panels: { idx: number; name: string; url: string }[] }
type HfHost = {
  id: string; remark: string; disabled: boolean; reality: boolean
  address: string[]; sni: string[]; host: string[]; skip: Record<string, boolean>; all: boolean
}

const FIELDS: [string, string][] = [['address', 'آدرس'], ['sni', 'SNI'], ['host', 'Host']]

function HfView() {
  const { data, err, reload } = useTool<HfData>('/api/tools/hf')
  const [cfg, setCfg] = useState<HfCfg | null>(null)
  const [pi, setPi] = useState(0)
  const [hosts, setHosts] = useState<HfHost[] | null>(null)
  const [hErr, setHErr] = useState('')
  const [notice, setNotice] = useState('')
  const [fErr, setFErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [logOpen, setLogOpen] = useState(false)
  const [log, setLog] = useState<any[] | null>(null)
  const [revOpen, setRevOpen] = useState(false)
  const [revs, setRevs] = useState<{ sid: string; from?: string; to?: string }[] | null>(null)
  const [bkOpen, setBkOpen] = useState(false)

  useEffect(() => {
    if (data?.cfg) setCfg(data.cfg)
  }, [data])
  useEffect(() => {
    if (!data?.panels?.length) return
    loadHosts(pi)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, pi])

  async function loadHosts(p: number) {
    setHErr('')
    try {
      const d = await api<{ hosts: HfHost[] }>(`/api/tools/hf/hosts?panel=${p}&t=${Date.now()}`)
      setHosts(d.hosts || [])
    } catch (e: any) {
      setHosts([])
      setHErr(e.message || 'خواندن هاست‌ها ناموفق')
    }
  }
  async function act(path: string, body: any, msg?: string) {
    setFErr('')
    try {
      await api(path, body)
      invalidateCache()
      if (msg) setNotice(msg)
      return true
    } catch (e: any) {
      setFErr(e.message || 'انجام نشد')
      return false
    }
  }
  if (err) return <Empty title="خطا" description={err} />
  if (!data || !cfg)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
    )
  const num = (k: keyof HfCfg, label: string) => (
    <label className="cc-field">
      <span>{label}</span>
      <Input type="number" value={String(cfg[k])} onChange={(e) => setCfg({ ...cfg, [k]: Number(e.target.value) || 0 } as HfCfg)} />
    </label>
  )
  const submit = async () => {
    if (await act('/api/tools/hf/cfg', cfg, 'تنظیمات ذخیره شد')) reload()
  }
  return (
    <>
      <StatGrid
        items={[
          { label: 'وضعیت', value: cfg.enabled ? 'فعال' : 'خاموش' },
          { label: 'فاصلهٔ اجرا', value: faNum(cfg.intervalMin) + ' دقیقه' },
          { label: 'استثناها', value: faNum(cfg.excCount) },
        ]}
      />
      <div className="cc-hint cc-mb">
        دامنهٔ هاست‌ها هر {faNum(cfg.intervalMin)} دقیقه از داخل ایران بررسی می‌شود؛ اگر فیلتر شد، ساب‌دامنهٔ تازه ساخته و
        جایگزین می‌شود. هاست‌هایی که لازم نیست بررسی شوند را در لیست پایین «استثنا» کن.
      </div>
      {cfg.lastSummary && <div className="cc-hint cc-center cc-mb">آخرین اجرا: {cfg.lastSummary}</div>}

      <div className="cc-form">
        <label className="cc-checkline">
          <input id="cgc4" name="cgc4" type="checkbox" checked={!!cfg.enabled} onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })} />
          <span>تعویض خودکار فعال باشد</span>
        </label>
        <div className="cc-field-row">
          {num('intervalMin', 'فاصلهٔ اجرا (دقیقه)')}
          {num('cities', 'حداقل شهر فیلتر')}
        </div>
        <div className="cc-field-row">
          {num('maxOk', 'حداکثر پینگ موفق (۰-۴)')}
          {num('batch', 'تعداد بررسی هر اجرا')}
        </div>
        <div className="cc-field-row">
          {num('maxChanges', 'حداکثر تعویض')}
          {num('recheckCount', 'تعداد بررسی مجدد')}
        </div>
        <div className="cc-field-row">
          {num('recheckMin', 'فاصلهٔ بررسی مجدد (دقیقه)')}
        </div>
        {fErr && <div className="cc-form-err">{fErr}</div>}
        <Button variant="primary" onClick={submit}>
          ذخیرهٔ تنظیمات
        </Button>
      </div>

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>هاست‌ها</h2>
          <Button
            variant="outline"
            disabled={busy}
            onClick={async () => {
              setBusy(true)
              const ok = await act('/api/tools/hf/run', {}, 'بررسی شروع شد؛ نتیجه در تلگرام می‌آید')
              setBusy(false)
              if (ok) setNotice('بررسی شروع شد؛ نتیجه در تلگرام می‌آید')
            }}
          >
            {busy ? 'در حال اجرا…' : 'بررسی الان'}
          </Button>
        </div>
        <div className="cc-btnrow">
          {data.panels.map((p) => (
            <Button key={p.idx} variant={p.idx === pi ? 'primary' : 'outline'} onClick={() => setPi(p.idx)}>
              {p.name}
            </Button>
          ))}
          <Button variant="outline" onClick={() => setBkOpen(true)}>
            بکاپ‌ها
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              setLogOpen(true)
              setLog(null)
              try {
                const d = await api<{ log: any[] }>(`/api/tools/hf/log?t=${Date.now()}`)
                setLog(d.log || [])
              } catch {
                setLog([])
              }
            }}
          >
            تاریخچه
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              if (await act('/api/tools/hf/exc', { panel: pi, clear: true })) loadHosts(pi)
            }}
          >
            پاک‌کردن استثناها
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              for (const h of hosts || []) {
                if (h.all) continue
                await act('/api/tools/hf/exc', { panel: pi, host: h.id, all: true })
              }
              loadHosts(pi)
            }}
          >
            استثنا کردن همه
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              setRevOpen(true)
              setRevs(null)
              try {
                const d = await api<{ items: any[] }>(`/api/tools/hf/revert?t=${Date.now()}`)
                setRevs(d.items || [])
              } catch {
                setRevs([])
              }
            }}
          >
            بازگردانی
          </Button>
        </div>
        {hErr && <div className="cc-form-err">{hErr}</div>}
        {!hosts && (
          <div className="cc-center">
            <Loader />
          </div>
        )}
        {hosts && hosts.length === 0 && <Empty title="هاستی نیست" description="این پنل هاست فعالی ندارد." />}
        {hosts?.map((h) => (
          <div key={h.id} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">
                {h.remark || h.id} {h.reality && <span className="cc-dim">· REALITY</span>}
              </div>
              <div className="cc-sub">{(h.address || []).join('، ') || '—'}</div>
              {(h.sni || []).length > 0 && <div className="cc-sub">sni: {(h.sni || []).join('، ')}</div>}
              {(h.host || []).length > 0 && <div className="cc-sub">host: {(h.host || []).join('، ')}</div>}
              <div className="cc-btnrow">
                {FIELDS.map(([f, l]) => (
                  <Button
                    key={f}
                    variant={h.skip[f] ? 'outline' : 'primary'}
                    onClick={async () => {
                      if (await act('/api/tools/hf/exc', { panel: pi, host: h.id, field: f })) loadHosts(pi)
                    }}
                  >
                    {l}
                  </Button>
                ))}

              </div>
            </div>
          </div>
        ))}
      </div>

      {bkOpen && (
        <BkDialog
          onClose={() => setBkOpen(false)}
          act={act}
          faTxt={(v) => v}
        />
      )}

      {logOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setLogOpen(false)}>
          <Dialog className="cc-dialog" size="base">
              <Dialog.Title className="cc-dialog-title">تاریخچهٔ تعویض‌ها</Dialog.Title>
              <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
              <div className="cc-dialog-body">
                {!log && (
                  <div className="cc-center">
                    <Loader />
                  </div>
                )}
                {log && log.length === 0 && <Empty title="خالی" description="چیزی ثبت نشده." />}
                {log && log.length > 0 && (
                  <div className="cc-form">
                    {[...log].reverse().map((e, i) => (
                      <div key={i} className="cc-sub cc-ltr">
                        {String(e.ts || '').replace('T', ' ').slice(0, 16)} · {e.kind} {e.from ? `${e.from} → ${e.to}` : ''}{' '}
                        {e.error ? `⚠️ ${e.error}` : ''}
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <div className="cc-dialog-actions">
                <Button variant="outline" onClick={() => setLogOpen(false)}>
                  بستن
                </Button>
              </div>
          </Dialog>
        </Dialog.Root>
      )}

      {revOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setRevOpen(false)}>
          <Dialog className="cc-dialog" size="base">
              <Dialog.Title className="cc-dialog-title">بازگردانی تعویض‌ها</Dialog.Title>
              <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
              <div className="cc-dialog-body">
                {!revs && (
                  <div className="cc-center">
                    <Loader />
                  </div>
                )}
                {revs && revs.length === 0 && <Empty title="خالی" description="تعویضی برای بازگردانی نیست." />}
                {revs &&
                  revs.map((r) => (
                    <div key={r.sid} className="cc-trow">
                      <div className="cc-grow">
                        <div className="cc-name cc-ltr">{r.from || r.sid}</div>
                        {r.to && <div className="cc-sub cc-ltr">→ {r.to}</div>}
                      </div>
                      <Button
                        variant="outline"
                        onClick={async () => {
                          if (await act('/api/tools/hf/revert', { sid: r.sid }, 'بازگردانی شد')) {
                            setRevOpen(false)
                            loadHosts(pi)
                          }
                        }}
                      >
                        بازگردان
                      </Button>
                    </div>
                  ))}
              </div>
              <div className="cc-dialog-actions">
                {fErr && <div className="cc-form-err">{fErr}</div>}
                <Button variant="outline" onClick={() => setRevOpen(false)}>
                  بستن
                </Button>
              </div>
          </Dialog>
        </Dialog.Root>
      )}
    </>
  )
}

// ---------------- pasarguard expiry / volume webhook ----------------
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
  if (err) return <Empty title="خطا" description={err} />
  if (!d)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
    )
  return (
    <>
      <StatGrid
        items={[
          { label: 'وضعیت', value: d.enabled ? 'فعال' : 'خاموش' },
          { label: 'پنل‌های متصل', value: faNum(d.panels.filter((p) => p.set).length) + '/' + faNum(d.panels.length) },
          { label: 'آستانهٔ حجم', value: (d.usage || []).map((u) => faNum(u) + '٪').join(' · ') || '—' },
        ]}
      />
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
        {d.panels.length === 0 && <Empty title="پنلی نیست" description="اول در بخش پاسارگارد یک پنل اضافه کن." />}
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
type HfdCfg = { enabled: boolean; intervalH: number; at: string; nextRun: string; totalCreated: number }
type HfdData = { cfg: HfdCfg; panels: { idx: number; name: string }[]; items: { key: string; panel: number; host: string }[] }
type HfdHost = { id: string; remark: string; disabled: boolean; reality: boolean; fields: Record<string, boolean> }

function HfdView() {
  const { data, err, reload } = useTool<HfdData>('/api/tools/hfd')
  const [cfg, setCfg] = useState<HfdCfg | null>(null)
  const [pi, setPi] = useState(0)
  const [hosts, setHosts] = useState<HfdHost[] | null>(null)
  const [hErr, setHErr] = useState('')
  const [notice, setNotice] = useState('')
  const [fErr, setFErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [logOpen, setLogOpen] = useState(false)
  const [log, setLog] = useState<any[] | null>(null)
  const [revOpen, setRevOpen] = useState(false)
  const [revs, setRevs] = useState<any[] | null>(null)

  useEffect(() => {
    if (data?.cfg) setCfg(data.cfg)
  }, [data])
  useEffect(() => {
    if (!data?.panels?.length) return
    loadHosts(pi)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, pi])

  async function loadHosts(p: number) {
    setHErr('')
    try {
      const d = await api<{ hosts: HfdHost[] }>(`/api/tools/hfd/hosts?panel=${p}&t=${Date.now()}`)
      setHosts(d.hosts || [])
    } catch (e: any) {
      setHosts([])
      setHErr(e.message || 'خواندن هاست‌ها ناموفق')
    }
  }
  async function act(path: string, body: any, msg?: string) {
    setFErr('')
    try {
      await api(path, body)
      invalidateCache()
      if (msg) setNotice(msg)
      return true
    } catch (e: any) {
      setFErr(e.message || 'انجام نشد')
      return false
    }
  }
  if (err) return <Empty title="خطا" description={err} />
  if (!data || !cfg)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
    )
  const next = cfg.nextRun ? faDate(cfg.nextRun) : '—'
  return (
    <>
      <StatGrid
        items={[
          { label: 'وضعیت', value: cfg.enabled ? 'فعال' : 'خاموش' },
          { label: 'هر', value: faNum(cfg.intervalH) + ' ساعت' },
          { label: 'اجرای بعدی', value: next },
          { label: 'هاست‌های انتخابی', value: faNum(data.items.length) },
        ]}
      />
      <div className="cc-hint cc-mb">
        هر {faNum(cfg.intervalH)} ساعت، ساب‌های هاست‌های انتخاب‌شده با نام تازه بازسازی می‌شوند تا فیلترها دور زده شوند.
        برای هر هاست یکی از سه فیلد «آدرس» (IP درخواست)، «SNI» (نام TLS) یا «Host» (هدر دامنه) را روشن کن.
      </div>

      <div className="cc-form">
        <label className="cc-checkline">
          <input id="cgc8" name="cgc8" type="checkbox" checked={!!cfg.enabled} onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })} />
          <span>چرخش روزانه فعال باشد</span>
        </label>
        <div className="cc-field-row">
          <label className="cc-field">
            <span>فاصله (ساعت)</span>
            <Input
              type="number"
              value={String(cfg.intervalH)}
              onChange={(e) => setCfg({ ...cfg, intervalH: Number(e.target.value) || 0 })}
            />
          </label>
          <label className="cc-field">
            <span>ساعت شروع (HH:MM)</span>
            <Input className="cc-input" value={cfg.at} onChange={(e) => setCfg({ ...cfg, at: e.target.value })} placeholder="03:00" />
          </label>
        </div>
        {cfg.totalCreated > 0 && <div className="cc-hint">تاکنون {faNum(cfg.totalCreated)} ساب ساخته شده.</div>}
        {fErr && <div className="cc-form-err">{fErr}</div>}
        <Button
          variant="primary"
          onClick={async () => {
            if (await act('/api/tools/hfd/cfg', cfg, 'تنظیمات ذخیره شد')) reload()
          }}
        >
          ذخیرهٔ تنظیمات
        </Button>
      </div>

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      <div className="cc-section">
        <div className="cc-section-head">
          <h2>هاست‌ها</h2>
          <Button
            variant="outline"
            disabled={busy}
            onClick={async () => {
              setBusy(true)
              const ok = await act('/api/tools/hfd/run', {}, 'چرخش شروع شد؛ نتیجه در تلگرام می‌آید')
              setBusy(false)
              if (!ok) setNotice('')
            }}
          >
            {busy ? 'در حال چرخش…' : 'چرخش الان'}
          </Button>
        </div>
        <div className="cc-btnrow">
          {data.panels.map((p) => (
            <Button key={p.idx} variant={p.idx === pi ? 'primary' : 'outline'} onClick={() => setPi(p.idx)}>
              {p.name}
            </Button>
          ))}
          <Button
            variant="outline"
            onClick={async () => {
              setLogOpen(true)
              setLog(null)
              try {
                const d = await api<{ log: any[] }>(`/api/tools/hfd/log?t=${Date.now()}`)
                setLog(d.log || [])
              } catch {
                setLog([])
              }
            }}
          >
            تاریخچه
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              setRevOpen(true)
              setRevs(null)
              try {
                const d = await api<{ items: any[] }>(`/api/tools/hfd/revert?t=${Date.now()}`)
                setRevs(d.items || [])
              } catch {
                setRevs([])
              }
            }}
          >
            بازگردانی
          </Button>
        </div>
        {hErr && <div className="cc-form-err">{hErr}</div>}
        {!hosts && (
          <div className="cc-center">
            <Loader />
          </div>
        )}
        {hosts && hosts.length === 0 && <Empty title="هاستی نیست" description="این پنل هاست فعالی ندارد." />}
        {hosts?.map((h) => (
          <div key={h.id} className="cc-trow">
            <div className="cc-grow">
              <div className="cc-name">
                {h.remark || h.id} {h.reality && <span className="cc-dim">· REALITY</span>}
              </div>
              <div className="cc-btnrow">
                {FIELDS.map(([f, l]) => (
                  <Button
                    key={f}
                    variant={h.fields[f] ? 'primary' : 'outline'}
                    onClick={async () => {
                      if (await act('/api/tools/hfd/field', { panel: pi, host: h.id, field: f })) loadHosts(pi)
                    }}
                  >
                    {l}
                  </Button>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>

      {logOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setLogOpen(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Title className="cc-dialog-title">تاریخچهٔ چرخش روزانه</Dialog.Title>
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <div className="cc-dialog-body">
              {!log && (
                <div className="cc-center">
                  <Loader />
                </div>
              )}
              {log && log.length === 0 && <Empty title="خالی" description="چیزی ثبت نشده." />}
              {log && log.length > 0 && (
                <div className="cc-form">
                  {log.map((e, i) => (
                    <div key={i} className="cc-sub cc-ltr">
                      {String(e.ts || '').replace('T', ' ').slice(0, 16)} · {e.kind} {e.remark ? `· ${e.remark}` : ''}{' '}
                      {e.error ? `⚠️ ${e.error}` : ''}
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="cc-dialog-actions">
              <Button variant="outline" onClick={() => setLogOpen(false)}>
                بستن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}

      {revOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setRevOpen(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Title className="cc-dialog-title">بازگردانی چرخش روزانه</Dialog.Title>
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <div className="cc-dialog-body">
              {!revs && (
                <div className="cc-center">
                  <Loader />
                </div>
              )}
              {revs && revs.length === 0 && <Empty title="خالی" description="چیزی برای بازگردانی نیست." />}
              {revs?.map((r) => (
                <div key={r.key} className="cc-trow">
                  <div className="cc-grow">
                    <div className="cc-name cc-ltr">{r.key}</div>
                    <div className="cc-sub">{faNum(r.prev)} مقدار قبلی ذخیره شده</div>
                  </div>
                  <Button
                    variant="outline"
                    onClick={async () => {
                      if (await act('/api/tools/hfd/revert', { panel: r.panel, host: r.host }, 'بازگردانی شد')) {
                        setRevOpen(false)
                        loadHosts(pi)
                      }
                    }}
                  >
                    بازگردان
                  </Button>
                </div>
              ))}
            </div>
            <div className="cc-dialog-actions">
              {fErr && <div className="cc-form-err">{fErr}</div>}
              <Button variant="outline" onClick={() => setRevOpen(false)}>
                بستن
              </Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
    </>
  )
}

// ---------------- quick record: add + search/bulk change ----------------
type CfZone = { id: string; name: string; acc: number; accName: string }
type CfRec = {
  id: string; type: string; name: string; content: string
  acc?: number; zid?: string; zone?: string; fav?: boolean; label?: string
}

function RecView() {
  const zones = useTool<{ zones: CfZone[] }>('/api/cf/zones')
  const [q, setQ] = useState('')
  const [hits, setHits] = useState<CfRec[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [sel, setSel] = useState<string[]>([])
  const [notice, setNotice] = useState('')
  const [err, setErr] = useState('')
  const [addOpen, setAddOpen] = useState(false)
  const [flt, setFlt] = useState('all')

  async function search() {
    if (!q.trim()) return
    setBusy(true)
    setErr('')
    setNotice('')
    setSel([])
    try {
      const d = await api<{ records: CfRec[] }>(`/api/cf/search?q=${encodeURIComponent(q.trim())}&t=${Date.now()}`)
      setHits(d.records || [])
    } catch (e: any) {
      setHits([])
      setErr(e.message || 'جست‌وجو ناموفق')
    }
    setBusy(false)
  }

  async function fav(r: CfRec) {
    try {
      await api('/api/cf/record', { op: 'fav', acc: r.acc, zid: r.zid, rid: r.id })
      setHits((h) => (h || []).map((x) => (x.id === r.id ? { ...x, fav: !x.fav } : x)))
    } catch (e: any) {
      setErr(e.message || 'انجام نشد')
    }
  }

  async function bulkDel() {
    if (!hits) return
    const groups: Record<string, { acc: number; zid: string; ids: string[] }> = {}
    for (const r of hits) {
      if (!sel.includes(r.id) || r.acc === undefined || !r.zid) continue
      const k = `${r.acc}|${r.zid}`
      groups[k] = groups[k] || { acc: r.acc, zid: r.zid, ids: [] }
      groups[k].ids.push(r.id)
    }
    let done = 0
    let failed = 0
    for (const g of Object.values(groups)) {
      try {
        const res = await api<{ ok?: number; fail?: number }>('/api/cf/bulk', { op: 'del', acc: g.acc, zid: g.zid, ids: g.ids })
        done += Number(res?.ok || g.ids.length)
        failed += Number(res?.fail || 0)
      } catch {
        failed += g.ids.length
      }
    }
    setNotice(`حذف شد: ${faNum(done)}${failed ? ` · ناموفق: ${faNum(failed)}` : ''}`)
    setSel([])
    invalidateCache()
    search()
  }

  const shown = (hits || []).filter((r) => flt !== 'fav' || !!r.fav)
  const toggle = (id: string) => setSel((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]))

  return (
    <>
      <div className="cc-pagebar">
        <Button variant="primary" icon={Plus} onClick={() => setAddOpen(true)}>
          افزودن رکورد
        </Button>
      </div>
      <div className="cc-hint cc-mb">
        یک آی‌پی، نام یا بخشی از آن را بفرست تا در همهٔ دامنه‌ها و اکانت‌ها بگردم؛ بعد می‌توانی چند رکورد را یک‌جا حذف یا منتخب کنی.
      </div>

      <FilterBar
        q={q}
        onQ={setQ}
        placeholder="آی‌پی، نام یا مقدار"
        onEnter={search}
        filters={[
          { id: 'all', label: 'همه' },
          { id: 'fav', label: 'منتخب' },
        ]}
        value={flt}
        onValue={setFlt}
        onRefresh={search}
        busy={busy}
      />
      {err && <ErrBox msg={err} />}
      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}

      {sel.length > 0 && (
        <div className="cc-bulkbar">
          <span>{faNum(sel.length)} انتخاب شده</span>
          <Button variant="outline" onClick={bulkDel}>
            حذف
          </Button>
          <Button variant="outline" onClick={() => setSel([])}>
            لغو
          </Button>
        </div>
      )}

      {busy && (
        <div className="cc-center">
          <Loader />
        </div>
      )}
      {hits && hits.length === 0 && !busy && <Empty title="چیزی پیدا نشد" description="عبارت دیگری را امتحان کن." />}
      {shown.map((r) => (
        <div key={r.id} className={'cc-trow cc-tap' + (sel.includes(r.id) ? ' sel' : '')}>
          <label className="cc-pick" onClick={(e) => e.stopPropagation()}>
            <input id="cgc9" name="cgc9" type="checkbox" checked={sel.includes(r.id)} onChange={() => toggle(r.id)} />
          </label>
          <div className="cc-grow">
            <div className="cc-name cc-ltr">
              {r.name} <Badge>{r.type}</Badge>
            </div>
            <div className="cc-sub cc-ltr">{r.content}</div>
            {r.zone && <div className="cc-sub">{r.zone}</div>}
          </div>
          <button className="cc-rowbtn" title="منتخب" onClick={(e) => { e.stopPropagation(); fav(r) }}>
            {r.fav ? '★' : '☆'}
          </button>
        </div>
      ))}

      {addOpen && zones.data && (
        <AddRecDialog
          zones={zones.data.zones || []}
          onClose={() => setAddOpen(false)}
          onDone={(m) => {
            setAddOpen(false)
            setNotice(m)
            invalidateCache()
            if (q.trim()) search()
          }}
        />
      )}
    </>
  )
}

function AddRecDialog({ zones, onClose, onDone }: { zones: CfZone[]; onClose: () => void; onDone: (m: string) => void }) {
  const [z, setZ] = useState('')
  const [type, setType] = useState('A')
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const [proxied, setProxied] = useState(true)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const zone = zones.find((x) => `${x.acc}|${x.id}` === z)
  async function submit() {
    if (!zone) return setErr('دامنه را انتخاب کن')
    if (!content.trim()) return setErr('مقدار رکورد لازم است')
    setBusy(true)
    setErr('')
    try {
      await api('/api/cf/record', { op: 'add', acc: zone.acc, zid: zone.id, type, name: name.trim(), content: content.trim(), proxied })
      onDone('رکورد اضافه شد')
    } catch (e: any) {
      setErr(e.message || 'افزودن ناموفق')
    }
    setBusy(false)
  }
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Title className="cc-dialog-title">افزودن رکورد</Dialog.Title>
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
            <div className="cc-field-row">
              <label className="cc-field">
                <span>نوع</span>
                <select className="cc-input" value={type} onChange={(e) => setType(e.target.value)}>
                  {['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'SRV'].map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </select>
              </label>
              <label className="cc-field">
                <span>نام (خالی = ریشه)</span>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="www" />
              </label>
            </div>
            <label className="cc-field">
              <span>مقدار</span>
              <Input value={content} onChange={(e) => setContent(e.target.value)} placeholder={type === 'A' ? '1.2.3.4' : '—'} />
            </label>
            <label className="cc-checkline">
              <input id="cgc10" name="cgc10" type="checkbox" checked={proxied} onChange={(e) => setProxied(e.target.checked)} />
              <span>پروکسی کلودفلر (فقط A/AAAA/CNAME)</span>
            </label>
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

// ---------------- domain email (catch-all forwarding) ----------------
type MailZone = { acc: number; zid: string; zname: string; dest?: string }
type MailData = { fwd: MailZone[]; worker: MailZone[] }

function MailView() {
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
  if (err) return <Empty title="خطا" description={err} />
  if (!data)
    return (
      <div className="cc-center">
        <Loader size="lg" />
      </div>
    )
  const all = [...data.fwd, ...data.worker]
  const shownFwd = data.fwd.filter((z) => flt !== 'worker' && (!q || z.zname.includes(q) || (z.dest || '').includes(q)))
  const shownWorker = data.worker.filter((z) => flt !== 'fwd' && (!q || z.zname.includes(q)))
  return (
    <>
      <StatGrid
        items={[
          { label: 'دامنه‌های وصل', value: faNum(all.length) },
          { label: 'فوروارد', value: faNum(data.fwd.length) },
          { label: 'ورکر', value: faNum(data.worker.length) },
        ]}
      />
      <div className="cc-pagebar">
        <Button variant="primary" icon={Plus} onClick={() => setOpen(true)}>
          افزودن ایمیل دامنه
        </Button>
      </div>
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
type SetData = { version: string; latest: string; isMain: boolean; admins: number[]; guides: { id: string; title: string; text: string }[] }

function SetView() {
  const { data, err, reload } = useTool<SetData>('/api/tools/settings')
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

function BkDialog({ onClose, act }: { onClose: () => void; act: (p: string, b: any, m?: string) => Promise<boolean>; faTxt: (v: string) => string }) {
  const [items, setItems] = useState<any[] | null>(null)
  const [busy, setBusy] = useState(false)
  const load = async () => {
    try {
      const d = await api<{ items: any[] }>(`/api/tools/hf/bk?t=${Date.now()}`)
      setItems(d.items || [])
    } catch {
      setItems([])
    }
  }
  useEffect(() => {
    load()
  }, [])
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Title className="cc-dialog-title">بکاپ هاست‌ها</Dialog.Title>
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <div className="cc-dialog-body">
          <div className="cc-btnrow cc-mb">
            <Button
              variant="primary"
              disabled={busy}
              onClick={async () => {
                setBusy(true)
                await act('/api/tools/hf/bk', { op: 'new' }, 'بکاپ گرفته شد')
                setBusy(false)
                load()
              }}
            >
              گرفتن بکاپ جدید
            </Button>
          </div>
          {!items && (
            <div className="cc-center">
              <Loader />
            </div>
          )}
          {items && items.length === 0 && <Empty title="بکاپی نیست" description="اولین بکاپ را بگیر." />}
          {items?.map((b) => (
            <div key={b.id} className="cc-trow">
              <div className="cc-grow">
                <div className="cc-name cc-ltr">{faDate(b.ts)}</div>
                <div className="cc-sub">
                  {b.label} · {faNum(b.count)} هاست
                </div>
              </div>
              <Button
                variant="outline"
                onClick={async () => {
                  if (await act('/api/tools/hf/bk', { op: 'restore', id: b.id }, 'بازگردانی انجام شد')) onClose()
                }}
              >
                بازگردان
              </Button>
              <button
                className="cc-rowbtn danger"
                onClick={async () => {
                  await act('/api/tools/hf/bk', { op: 'del', id: b.id }, 'حذف شد')
                  load()
                }}
              >
                <Trash size={16} />
              </button>
            </div>
          ))}
        </div>
        <div className="cc-dialog-actions">
          <Button variant="outline" onClick={onClose}>
            بستن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

// verGt reports whether version a is newer than b (both "x.y.z" or "vx.y.z").
function verGt(a: string, b: string) {
  const pa = String(a).replace(/^v/, '').split('.').map((x) => Number(x) || 0)
  const pb = String(b).replace(/^v/, '').split('.').map((x) => Number(x) || 0)
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] || 0) - (pb[i] || 0)
    if (d) return d > 0
  }
  return false
}

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
  const ownerTxt = d?.owner === 'worker' ? 'ورکر پشتیبان' : d?.owner === 'other' ? 'نامشخص' : 'سرور'

  return (
    <>
      <StatGrid
        items={[
          { label: 'وضعیت فیلور', value: d?.configured ? (d.enabled ? 'فعال' : 'خاموش') : 'تنظیم نشده' },
          { label: 'سرور', value: d?.serverUp ? 'سالم' : 'قطع' },
          { label: 'مسئول وبهوک', value: ownerTxt },
        ]}
      />

      <div className="cc-pagebar">
        <Button variant="primary" icon={Plus} onClick={() => { setFErr(''); setForm({ token: '', name: '' }) }}>
          {d?.configured ? 'تنظیم مجدد / تغییر ورکر' : 'راه‌اندازی فیلور'}
        </Button>
      </div>

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}
      {err && <ErrBox msg={err} onRetry={reload} />}
      {!loaded && <Skeleton rows={3} />}

      {d && d.configured && (
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

type HubStats = { bot: string; version: string; go: string; uptime: string; mem_mb: number; db_mb: number; admins: number; users: number; panels: number; zones: number; cf_accs: number; servers: number }
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
          { label: 'نسخه', value: s?.version || '—' },
          { label: 'آپ‌تایم', value: s?.uptime || '—' },
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
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">کاربران / ادمین‌ها</div><div className="cc-sub">{faNum(s.users)} / {faNum(s.admins)}</div></div></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">پنل‌ها / سرورها</div><div className="cc-sub">{faNum(s.panels)} / {faNum(s.servers)}</div></div></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">دامنه‌ها</div><div className="cc-sub">{faNum(s.zones)} در {faNum(s.cf_accs)} اکانت</div></div></div>
          <div className="cc-trow"><div className="cc-grow"><div className="cc-name">حافظه / دیتابیس</div><div className="cc-sub">{s.mem_mb.toFixed(1)} MB / {s.db_mb.toFixed(2)} MB</div></div></div>
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

type QaHit = { acc: number; zid: string; zname: string; rid: string; name: string; type: string }
type QuotaData = { limit: number; autoStop: boolean; alerts: boolean; paused: boolean; pausedTill: number; usage: string[] }

function QaView() {
  const [q, setQ] = useState('')
  const [ip, setIp] = useState('')
  const [hits, setHits] = useState<QaHit[] | null>(null)
  const [sel, setSel] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [add, setAdd] = useState<{ acc: number; zid: string; name: string } | null>(null)
  const [zones, setZones] = useState<{ acc: number; id: string; name: string }[]>([])
  const [quota, setQuota] = useState<QuotaData | null>(null)
  const [limit, setLimit] = useState('')

  useEffect(() => {
    api<QuotaData>('/api/tools/quota').then((d) => { setQuota(d); setLimit(String(d.limit)) }).catch(() => {})
  }, [])

  const search = async () => {
    const v = q.trim()
    if (!v) return
    setBusy(true); setNotice(''); setSel([])
    try {
      const d = await api<{ ip: string; hits: QaHit[] }>('/api/tools/qa/search?ip=' + encodeURIComponent(v))
      setIp(d.ip); setHits(d.hits || [])
    } catch (e: any) {
      setNotice(String(e?.message || e))
    } finally { setBusy(false) }
  }

  const key = (h: QaHit) => h.acc + '|' + h.rid
  const chosen = () => (hits || []).filter((h) => sel.includes(key(h)))
  const allOn = !!hits?.length && sel.length === hits.length

  return (
    <>
      <StatGrid items={[{ label: 'رکوردهای یافته', value: hits ? faNum(hits.length) : '—' }, { label: 'انتخاب‌شده', value: faNum(sel.length) }, { label: 'آی‌پی', value: ip || '—' }]} />

      <FilterBar
        q={q}
        onQ={setQ}
        onEnter={search}
        placeholder="آی‌پی یا نام را بفرست…"
        onRefresh={search}
        busy={busy}
      />

      {notice && <div className="cc-hint cc-center cc-mb">{notice}</div>}
      {hits && hits.length === 0 && <Empty title="چیزی پیدا نشد" description="این آی‌پی در هیچ رکوردی از اکانت‌های کلادفلر نیست." />}

      {hits && hits.length > 0 && (
        <>
          <div className="cc-bulkbar">
            <label className="cc-checkline" style={{ padding: 0, background: 'transparent' }}>
              <input id="cgc11" name="cgc11" type="checkbox" checked={allOn} onChange={() => setSel(allOn ? [] : hits.map(key))} />
              <span>{allOn ? 'همه' : 'انتخاب همه'}</span>
            </label>
            <Button variant="outline" disabled={!sel.length} onClick={async () => {
              const d = await api<{ added: number }>('/api/tools/qa/fav', { hits: chosen() })
              setNotice(`${faNum(d.added)} رکورد منتخب شد`)
            }}>منتخب</Button>
            <Button variant="outline" icon={Plus} onClick={() => setAdd({ acc: hits[0].acc, zid: hits[0].zid, name: '' })}>افزودن ساب</Button>
            <Button variant="destructive" disabled={!sel.length} onClick={async () => {
              if (!confirm('رکوردهای انتخاب‌شده حذف شوند؟')) return
              const d = await api<{ ok: number; fail: number }>('/api/tools/qa/del', { hits: chosen() })
              setNotice(`${faNum(d.ok)} حذف شد` + (d.fail ? ` · ${faNum(d.fail)} ناموفق` : ''))
              setHits((hits || []).filter((h) => !sel.includes(key(h))))
              setSel([])
            }}>حذف</Button>
          </div>

          {hits.map((h) => (
            <div key={key(h)} className={'cc-trow cc-tap' + (sel.includes(key(h)) ? ' sel' : '')} onClick={() => setSel(sel.includes(key(h)) ? sel.filter((x) => x !== key(h)) : [...sel, key(h)])}>
              <label className="cc-pick" onClick={(e) => e.stopPropagation()}>
                <input id="cgc12" name="cgc12" type="checkbox" checked={sel.includes(key(h))} onChange={() => setSel(sel.includes(key(h)) ? sel.filter((x) => x !== key(h)) : [...sel, key(h)])} />
              </label>
              <div className="cc-grow">
                <div className="cc-name">{h.name}</div>
                <div className="cc-sub">{h.type} · {h.zname}</div>
              </div>
            </div>
          ))}
        </>
      )}

      <div className="cc-section">
        <div className="cc-section-head"><h2>سهمیه و مصرف</h2></div>
        <div className="cc-trow">
          <div className="cc-grow"><div className="cc-name">سقف درخواست روزانه</div><div className="cc-sub">هشدار و توقف خودکار بالای این عدد</div></div>
          <Input className="cc-input cc-num" value={limit} onChange={(e) => setLimit(e.target.value)} />
        </div>
        <div className="cc-checkline cc-mb"><input id="cgc13" name="cgc13" type="checkbox" checked={!!quota?.autoStop} onChange={(e) => setQuota(quota ? { ...quota, autoStop: e.target.checked } : quota)} /><span>توقف خودکار کرون‌ها</span></div>
        <div className="cc-checkline cc-mb"><input id="cgc14" name="cgc14" type="checkbox" checked={!!quota?.alerts} onChange={(e) => setQuota(quota ? { ...quota, alerts: e.target.checked } : quota)} /><span>هشدار تلگرام</span></div>
        <div className="cc-btnrow">
          <Button variant="primary" onClick={async () => {
            await save('/api/tools/quota/cfg', { limit: Number(limit) || 0, autoStop: !!quota?.autoStop, alerts: !!quota?.alerts }, () => {})
            setNotice('سهمیه ذخیره شد')
          }}>ذخیره</Button>
          <Button variant="outline" icon={Power} onClick={async () => {
            await save('/api/tools/quota/cfg', { pause: !quota?.paused }, () => {})
            const d = await api<QuotaData>('/api/tools/quota'); setQuota(d)
            setNotice(d.paused ? 'کرون‌ها ۲۴ ساعت متوقف شدند' : 'کرون‌ها فعال شدند')
          }}>{quota?.paused ? 'ادامهٔ کرون‌ها' : 'توقف ۲۴ ساعته'}</Button>
        </div>
      </div>

      {add && (
        <FormDialog
          title="افزودن ساب"
          sub={ip ? 'برای آی‌پی ' + ip : ''}
          onClose={() => setAdd(null)}
          onSave={async () => {
            try {
              await api('/api/tools/qa/add', { acc: add.acc, zid: add.zid, name: add.name, ip })
              setAdd(null); setNotice('رکورد ساخته شد'); search()
            } catch (e: any) { setNotice(String(e?.message || e)) }
          }}
        >
          <label className="cc-field">
            <span>دامنه</span>
            <select className="cc-input" value={add.zid} onChange={(e) => setAdd({ ...add, zid: e.target.value })} onFocus={async () => {
              if (zones.length) return
              const d = await api<{ zones: any[] }>('/api/cf/zones')
              setZones((d.zones || []).map((z: any) => ({ acc: z.acc, id: z.id, name: z.name })))
            }}>
              {zones.length === 0 && <option value={add.zid}>در حال بارگذاری…</option>}
              {zones.map((z) => <option key={z.id} value={z.id}>{z.name}</option>)}
            </select>
          </label>
          <label className="cc-field"><span>نام ساب</span><Input value={add.name} onChange={(e) => setAdd({ ...add, name: e.target.value })} placeholder="test" /></label>
        </FormDialog>
      )}
    </>
  )
}
