import { useEffect, useState } from 'react'
import { Pulse, ArrowClockwise, Copy, Gear, HardDrive, Info, PencilSimple, Plus, Power, ShieldCheck, Trash } from '@phosphor-icons/react'
import { api, faNum, fallbackCopy, invalidateCache, timeAgo } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from '../components/ui'
import { StatGrid } from '../components/Stats'
import { FilterBar } from '../components/FilterBar'
import { usePageAction } from '../components/PageAction'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { FormDialog } from './shared'
import { useTool, save, } from './shared'
import type { Srv, } from './shared'

function SrvView() {
  usePageAction({ label: 'افزودن سرور', icon: Plus, onClick: () => setEdit({ port: 22, user: 'root' }) })

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
  if (err) return <ErrBox msg={err} onRetry={reload} />
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
          { label: 'سرورها', value: list.length, note: 'ثبت‌شده' },
          { label: 'با کلید', value: list.filter((s) => s.key).length, of: list.length },
          { label: 'با رمز', value: list.filter((s) => !s.key).length, of: list.length },
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
            <div className="cc-sub">{s.last?.ts ? 'آخرین بررسی ' + timeAgo(s.last.ts) : 'بررسی نشده'}</div>
          </div>
          <span className={'cc-st' + (s.last?.ts ? (s.last.ok ? ' up' : ' down') : '')} title={s.last?.ts ? (s.last.ok ? 'آنلاین' : s.last.err || 'قطع') : 'وضعیت نامشخص'} />
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
          disabled={!edit.host}
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

export default SrvView
