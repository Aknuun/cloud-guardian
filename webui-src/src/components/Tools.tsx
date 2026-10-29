import { useEffect, useState } from 'react'
import { BellRinging, Check, PencilSimple, Plus, ShieldCheck, Terminal, Trash } from '@phosphor-icons/react'
import type { Icon } from '@phosphor-icons/react'
import { api, faDate, faNum, invalidateCache } from '../lib'
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

function FormDialog({
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
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const list = data || []

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
      {list.length === 0 && <Empty title="سروری ثبت نشده" description="با دکمهٔ «افزودن سرور» یکی اضافه کن." />}
      {list.map((s) => (
        <div key={s.id} className="cc-trow">
          <div className="cc-avatar">{(s.name || 'S').slice(0, 1).toUpperCase()}</div>
          <div className="cc-grow">
            <div className="cc-name">{s.name || s.host}</div>
            <div className="cc-sub">
              {s.user}@{s.host}:{s.port}
            </div>
            {s.note && <div className="cc-sub">{s.note}</div>}
          </div>
          <Badge>{s.key ? 'کلید' : 'رمز'}</Badge>
          <div className="cc-rowbtns">
            <button type="button" aria-label="ویرایش" onClick={() => setEdit({ ...s, secret: '' })}>
              <PencilSimple size={17} />
            </button>
            <button type="button" aria-label="حذف" className="danger" onClick={() => setDel(s)}>
              <Trash size={17} />
            </button>
          </div>
        </div>
      ))}

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
              placeholder="رمز عبور، یا کلید با -----BEGIN"
              dir="ltr"
            />
          </label>
          <label className="cc-field">
            <span>یادداشت</span>
            <Input value={edit.note || ''} onChange={(e) => setEdit({ ...edit, note: e.target.value })} />
          </label>
        </FormDialog>
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
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const list = data || []
  const pending = list.filter((r) => !r.ack).length

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
      {list.length === 0 && <Empty title="یادآوری نیست" description="با دکمهٔ «یادآور جدید» یکی بساز." />}
      {list.map((r) => (
        <div key={r.id} className="cc-trow">
          <div className="cc-grow">
            <div className="cc-name">{r.text}</div>
            <div className="cc-sub">
              {r.when > 0 ? faDate(Number(r.when)) : 'هر وقت آنلاین شدی'}
              {r.server ? ' · ' + r.server : ''}
            </div>
          </div>
          <Badge>{r.ack ? 'انجام شد' : r.notified ? 'اطلاع داده شد' : 'در انتظار'}</Badge>
          <div className="cc-rowbtns">
            {!r.ack && (
              <button
                type="button"
                aria-label="انجام شد"
                onClick={async () => {
                  await save('/api/tools/rem/done', { id: r.id }, reload)
                }}
              >
                <Check size={17} />
              </button>
            )}
            <button
              type="button"
              aria-label="حذف"
              className="danger"
              onClick={async () => {
                await save('/api/tools/rem/del', { id: r.id }, reload)
              }}
            >
              <Trash size={17} />
            </button>
          </div>
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
  if (err) return <Empty title="خطا" description={err} />
  if (!loaded) return <Loader size="lg" />
  const iso = data || { panels: [], monitors: [] }
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
