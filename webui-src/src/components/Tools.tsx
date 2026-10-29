import { useEffect, useState } from 'react'
import { Pulse, ArrowClockwise, BellRinging, Check, Copy, Gear, HardDrive, Info, PencilSimple, Plus, Power, ShieldCheck, Terminal, Trash } from '@phosphor-icons/react'
import type { Icon } from '@phosphor-icons/react'
import { api, faDate, faNum, fallbackCopy, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from './ui'
import { StatGrid } from './Stats'

export type ToolId = 'srv' | 'rem' | 'pg'

export const TOOLS: { id: ToolId; label: string; Icon: Icon }[] = [
  { id: 'srv', label: 'سرورها و مانیتورینگ', Icon: Terminal },
  { id: 'rem', label: 'یادآورها', Icon: BellRinging },
  { id: 'pg', label: 'پاسارگارد', Icon: ShieldCheck },
]

export type ToolItem = { id: ToolId; label: string; Icon: Icon; active?: boolean; onSelect: () => void }

export function toolItems(active: ToolId | null, onSelect: (id: ToolId) => void): ToolItem[] {
  return TOOLS.map((t) => ({ ...t, active: active === t.id, onSelect: () => onSelect(t.id) }))
}

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
type Rem = { id: string; text: string; when: number; notified: boolean; ack: boolean; server: string }
type PgData = {
  panels: { name: string; url: string; user: string }[]
  monitors: { id: string; name: string; panel: string; interval: number; enabled: boolean }[]
}

export function Tools({ id }: { id: ToolId }) {
  if (id === 'srv') return <SrvView />
  if (id === 'rem') return <RemView />
  return <PgView />
}

export function FormDialog({
  title,
  sub,
  onClose,
  onSave,
  children,
  error,
}: {
  title: string
  sub?: string
  onClose: () => void
  onSave: () => void
  children: React.ReactNode
  error?: string
}) {
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
        <Dialog.Title className="cc-dialog-title">{title}</Dialog.Title>
        {sub && <Dialog.Description className="cc-dialog-desc">{sub}</Dialog.Description>}
        <div className="cc-form">{children}</div>
        {error && <div className="cc-form-err">{error}</div>}
        <div className="cc-dialog-actions">
          <Button variant="secondary" onClick={onClose}>
            انصراف
          </Button>
          <Button variant="primary" onClick={onSave}>
            ذخیره
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
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
  const [sel, setSel] = useState<string[]>([])
  const [bulkDel, setBulkDel] = useState(false)

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
  if (!loaded) return <Loader size="lg" />
  const list = data || []
  const needle = q.trim().toLowerCase()
  const shown = needle
    ? list.filter((s) => (s.name + ' ' + s.host + ' ' + s.user + ' ' + s.note).toLowerCase().includes(needle))
    : list
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
      </div>
      <div className="cc-btnrow">
        <Button variant="outline" onClick={() => setImpOpen(true)}>
          افزودن گروهی
        </Button>
        <Button variant="outline" onClick={() => setSubOpen(true)}>
          از ساب‌دامنه
        </Button>
        <Button variant="outline" onClick={() => setPwOpen(true)}>
          رمزهای ذخیره‌شده
        </Button>
      </div>
      <div className="cc-searchbar">
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="جستجو در سرورها…" />
        <Button variant="outline" icon={Gear} onClick={() => setCfgOpen(true)}>
          آستانه‌ها
        </Button>
      </div>
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
            <input type="checkbox" checked={sel.includes(s.id)} onChange={() => toggle(s.id)} aria-label="انتخاب سرور" />
          </label>
          <div className="cc-avatar">{(s.name || 'S').slice(0, 1).toUpperCase()}</div>
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
                    <b>{Math.round(Number(monData.cpu?.pct) || 0)}%</b>
                    <i>{monData.cpu?.cores ? monData.cpu.cores + ' هسته' : '—'}</i>
                  </div>
                  <div>
                    <span>RAM</span>
                    <b>{Math.round(Number(monData.mem?.pct) || 0)}%</b>
                    <i>
                      {monData.mem?.usedMb ?? '—'} / {monData.mem?.totalMb ?? '—'} MB
                    </i>
                  </div>
                  <div>
                    <span>دیسک</span>
                    <b>{monData.disk?.pct || '—'}</b>
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
  const [server, setServer] = useState('')
  const [formErr, setFormErr] = useState('')
  const [q, setQ] = useState('')
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const list = data || []
  const pending = list.filter((r) => !r.ack).length
  const needle = q.trim().toLowerCase()
  const shown = needle ? list.filter((r) => (r.text + ' ' + r.server).toLowerCase().includes(needle)) : list

  const submit = async () => {
    setFormErr('')
    try {
      await save('/api/tools/rem/add', { text, when, server }, () => {
        setAdd(false)
        setText('')
        setWhen('')
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
      <div className="cc-searchbar">
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="جستجو در یادآورها…" />
      </div>
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
          <label className="cc-field">
            <span>تاریخ و ساعت (شمسی)</span>
            <Input
              value={when}
              onChange={(e) => setWhen(e.target.value)}
              placeholder="1404/08/07 14:30 — خالی: وقتی آنلاین شدم"
              dir="ltr"
            />
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
  const { data, err, loaded } = useTool<PgData>('/api/tools/pg')
  const [q, setQ] = useState('')
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const iso0 = data || { panels: [], monitors: [] }
  const needle = q.trim().toLowerCase()
  const iso = needle
    ? {
        panels: iso0.panels.filter((p) => (p.name + ' ' + p.url + ' ' + p.user).toLowerCase().includes(needle)),
        monitors: iso0.monitors.filter((m) => (m.name + ' ' + m.panel).toLowerCase().includes(needle)),
      }
    : iso0
  const on = iso.monitors.filter((m) => m.enabled).length
  return (
    <>
      <StatGrid
        items={[
          { label: 'پنل‌ها', value: faNum(iso.panels.length) },
          { label: 'مانیتور نود', value: faNum(iso.monitors.length) },
          { label: 'فعال', value: faNum(on) },
        ]}
      />
      <div className="cc-searchbar">
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="جستجو در پاسارگارد…" />
      </div>
      <div className="cc-section">پنل‌ها</div>
      {iso.panels.length === 0 && <Empty title="پنلی ثبت نشده" description="از ربات پنل پاسارگارد اضافه کن." />}
      {iso.panels.map((p) => (
        <div key={p.url} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{p.name || p.url}</div>
            <div className="cc-sub cc-mono">{p.url}</div>
          </div>
          {p.user && <Badge>{p.user}</Badge>}
        </div>
      ))}
      <div className="cc-section">مانیتور نودها</div>
      {iso.monitors.length === 0 && <Empty title="مانیتوری نیست" description="از ربات مانیتور نود بساز." />}
      {iso.monitors.map((m) => (
        <div key={m.id} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{m.name || m.id}</div>
            <div className="cc-sub">
              {m.panel} · هر {faNum(m.interval)} دقیقه
            </div>
          </div>
          <Badge>{m.enabled ? 'فعال' : 'خاموش'}</Badge>
        </div>
      ))}
    </>
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
        {err && <div className="cc-form-err">{err}</div>}
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
        {err && <div className="cc-form-err">{err}</div>}
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
        {err && <div className="cc-form-err">{err}</div>}
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
        {err && <div className="cc-form-err">{err}</div>}
        {!zones && (
          <div className="cc-center cc-mb">
            <Loader size="lg" />
          </div>
        )}
        {zones && !zone && (
          <div className="cc-form">
            {zones.length === 0 && <div className="cc-hint">زونی در کلادفلر نیست.</div>}
            {zones.map((z) => (
              <button key={z.id} type="button" className="cc-choice" onClick={() => pickZone(z)}>
                <span className="cc-choice-label">{z.name}</span>
              </button>
            ))}
          </div>
        )}
        {zone && (
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
          <Button variant="primary" onClick={submit} disabled={!zone || busy}>
            افزودن
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}
