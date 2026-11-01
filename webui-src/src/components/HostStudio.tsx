import { useEffect, useMemo, useState } from 'react'
import { ArrowClockwise, CaretLeft, Check, Copy, Download, Lock, PencilSimple, Plus, ShieldWarning, Trash, Upload } from '@phosphor-icons/react'
import { api, faDate, faNum, invalidateCache } from '../lib'
import { Badge, Button, Dialog, Empty, Input, Loader } from './ui'
import { FormDialog, ErrBox } from './toolsMeta'
import { SectionHeader } from './SectionHeader'
import { ListRow } from './ui-kit'
import { usePageAction } from './PageAction'

type Group = { id: string; label: string; emoji: string; service: string }
type Anchor = { enabled: boolean; slot: string; host_id: any }
type Tpl = {
  id: string
  name: string
  description: string
  is_default: boolean
  scope: string
  segments: string[]
  variables: string[]
  fonts: Record<string, string>
  emojis: Record<string, string>
  separator: string
  spacing: string
  group_order: string[]
  anchor: Anchor
  group_overrides: Record<string, any>
  host_overrides: Record<string, any>
  version: number
}
type HHost = {
  panel: number
  id: string
  remark: string
  group: string
  groupName: string
  priority: number
  disabled: boolean
  excluded: boolean
  locked: boolean
}
type Ov = {
  panel: number
  host_id: string
  excluded: boolean
  locked_name: boolean
  locked_priority: boolean
  custom_remark: string
  reason: string
  enabled: boolean
}
type Row = {
  panel: number
  id: string
  remark: string
  new: string
  priority: number
  newPriority: number
  group: string
  groupName: string
  locked: boolean
  excluded: boolean
  disabled: boolean
  changed: boolean
  warnings?: string[]
}
type Data = {
  panels: { idx: number; name: string; url: string }[]
  templates: Tpl[]
  hosts: HHost[]
  overrides: Ov[]
  backups: { id: string; ts: number; panel: number; count: number; hash: string }[]
  audit: { ts: number; uid: number; panel: number; template: string; hosts: number; ok: boolean; note?: string }[]
  groups: Group[]
  error?: string
}

const FONTS = [
  { v: 'sans', l: 'معمولی' },
  { v: 'mono', l: 'مونو' },
  { v: 'bold', l: 'بولد' },
  { v: 'serif', l: 'ایتالیک' },
]
const VARS = ['{flag}', '{group_icon}', '{group_name}', '{protocol}', '{transport}', '{address}', '{port}', '{sni}', '{path}', '{USERNAME}', '{STATUS_EMOJI}', '{TIME_LEFT}', '{DATA_LEFT}', '{service}', '{noads}', '{priority}', '{host_id}', '{inbound_tag}']
const TABS = [
  { id: 'tpl', l: 'قالب‌ها' },
  { id: 'hosts', l: 'هاست‌ها' },
  { id: 'prev', l: 'پیش‌نمایش' },
  { id: 'exc', l: 'استثناها' },
  { id: 'hist', l: 'تاریخچه' },
]

export function HostStudio() {
  const [panel, setPanel] = useState(0)
  const [tab, setTab] = useState('tpl')
  const [data, setData] = useState<Data | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [edit, setEdit] = useState<Tpl | null>(null)
  const [editErr, setEditErr] = useState('')
  const [q, setQ] = useState('')
  const [sel, setSel] = useState<string[]>([])
  const [tplId, setTplId] = useState('')
  const [mode, setMode] = useState('both')
  const [rows, setRows] = useState<Row[] | null>(null)
  const [applyOpen, setApplyOpen] = useState(false)
  const [applyWord, setApplyWord] = useState('')
  const [force, setForce] = useState(false)
  const [hv, setHv] = useState<HHost | null>(null)
  const [ov, setOv] = useState<Ov | null>(null)
  usePageAction({ label: 'قالب جدید', icon: Plus, disabled: !data, onClick: () => data && setEdit(blankTpl(data)) })

  function load() {
    setErr('')
    api<Data>('/api/tools/hs?panel=' + panel + '&t=' + Date.now())
      .then((d) => {
        setData(d)
        if (!tplId) {
          const def = d.templates.find((t) => t.is_default) || d.templates[0]
          if (def) setTplId(def.id)
        }
        if (d.error) setErr(d.error)
      })
      .catch((e) => setErr(e.message || 'خطا'))
  }
  useEffect(load, [panel])

  const groups = data?.groups || []
  const shown = useMemo(() => {
    const list = data?.hosts || []
    const s = q.trim().toLowerCase()
    if (!s) return list
    return list.filter((h) => (h.remark + ' ' + h.id + ' ' + h.groupName).toLowerCase().includes(s))
  }, [data, q])

  async function act(path: string, body: any, done?: (d: any) => void) {
    setBusy(true)
    setNotice('')
    try {
      const d = await api<any>(path, body)
      invalidateCache()
      if (done) done(d)
      return d
    } catch (e: any) {
      setNotice(e.message || 'خطا')
      return null
    } finally {
      setBusy(false)
    }
  }

  function preview() {
    return act('/api/tools/hs/preview', { panel, template: tplId, mode, ids: sel, force }, (d) => {
      setRows(d.rows || [])
      setTab('prev')
    })
  }

  function apply() {
    return act('/api/tools/hs/apply', { panel, template: tplId, mode, ids: sel, force, confirm: applyWord }, (d) => {
      setApplyOpen(false)
      setApplyWord('')
      setRows(null)
      setNotice('✅ ' + faNum(d.changed) + ' هاست تغییر کرد')
      load()
    })
  }

  function openHost(h: HHost) {
    const o = (data?.overrides || []).find((x) => x.panel === h.panel && x.host_id === h.id)
    setHv(h)
    setOv(
      o || {
        panel: h.panel,
        host_id: h.id,
        excluded: false,
        locked_name: false,
        locked_priority: false,
        custom_remark: '',
        reason: '',
        enabled: true,
      },
    )
  }

  function saveOv() {
    if (!ov) return
    return act('/api/tools/hs/ov', { op: 'save', ...ov }, () => {
      setHv(null)
      setOv(null)
      load()
    })
  }

  function exportTpls() {
    const blob = new Blob([JSON.stringify({ templates: data?.templates || [] }, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'host-studio-templates.json'
    a.click()
    URL.revokeObjectURL(a.href)
  }

  function importTpls(f: File | null) {
    if (!f) return
    const rd = new FileReader()
    rd.onload = () => {
      try {
        const j = JSON.parse(String(rd.result))
        const list = Array.isArray(j) ? j : j.templates
        if (!Array.isArray(list)) throw new Error('قالب معتبر نیست')
        act('/api/tools/hs/tpl', { op: 'import', templates: list }, () => load())
      } catch (e: any) {
        setNotice(e.message || 'فایل معتبر نیست')
      }
    }
    rd.readAsText(f)
  }

  return (
    <div className="cc-hs">
      <div className="cc-hschips">
        {(data?.panels || []).map((p) => (
          <button key={p.idx} className={'cc-chip' + (panel === p.idx ? ' on' : '')} onClick={() => { setPanel(p.idx); setRows(null) }}>
            {p.name || p.url}
          </button>
        ))}
        <Button variant="outline" icon={ArrowClockwise} onClick={load}>به‌روزرسانی</Button>
      </div>

      <div className="cc-hstabs" role="tablist">
        {TABS.map((t) => (
          <button key={t.id} role="tab" aria-selected={tab === t.id} className={'cc-hstab' + (tab === t.id ? ' on' : '')} onClick={() => setTab(t.id)}>
            {t.l}
          </button>
        ))}
      </div>

      {!data && !err && <Loader />}
      {err && <ErrBox msg={err} onRetry={load} />}
      {notice && <div className="cc-hint cc-mb">{notice}</div>}

      {data && tab === 'tpl' && (
        <>
          <div className="cc-hsbar">
            <Button variant="outline" icon={Download} onClick={exportTpls}>خروجی</Button>
            <label className="cc-filebtn">
              <Upload size={16} /> ورود قالب
              <input type="file" accept="application/json" onChange={(e) => importTpls(e.target.files?.[0] || null)} />
            </label>
          </div>
          <div className="cc-tplgrid">
            {data.templates.map((t) => (
              <div key={t.id} className={'cc-tpl' + (t.is_default ? ' def' : '')}>
                <div className="cc-tplhead">
                  <b>{t.name}</b>
                  {t.is_default && <Badge variant="success">پیش‌فرض</Badge>}
                </div>
                <div className="cc-tplprev">{(t.segments || []).join(t.separator || ' | ')}</div>
                <div className="cc-sub">{t.description}</div>
                <div className="cc-tplbtns">
                  {!t.is_default && (
                    <Button variant="ghost" onClick={() => act('/api/tools/hs/tpl', { op: 'default', id: t.id }, () => load())}>
                      پیش‌فرض
                    </Button>
                  )}
                  <Button variant="ghost" onClick={() => act('/api/tools/hs/tpl', { op: 'dup', id: t.id }, () => load())}>
                    <Copy size={15} /> کپی
                  </Button>
                  <Button variant="ghost" onClick={() => setEdit({ ...t })}>
                    <PencilSimple size={15} /> ویرایش
                  </Button>
                  <Button variant="ghost" onClick={() => act('/api/tools/hs/tpl', { op: 'del', id: t.id }, () => load())}>
                    <Trash size={15} />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {data && tab === 'hosts' && (
        <>
          <div className="cc-searchbar">
            <Input placeholder="جستجوی نام، شناسه یا گروه…" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          {sel.length > 0 && (
            <div className="cc-bulkbar">
              <span>{faNum(sel.length)} انتخاب شده</span>
              <Button variant="secondary" onClick={preview}>پیش‌نمایش</Button>
              <Button variant="ghost" onClick={() => setSel([])}>لغو انتخاب</Button>
            </div>
          )}
          <div className="cc-trowlist">
            {shown.map((h) => {
              const on = sel.includes(h.id)
              return (
                <div key={h.id} className={'cc-trow cc-tap' + (on ? ' sel' : '')} onClick={() => openHost(h)}>
                  <label className="cc-pick" onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      checked={on}
                      onChange={() => setSel((s) => (on ? s.filter((x) => x !== h.id) : [...s, h.id]))}
                    />
                  </label>
                  <div className="cc-grow">
                    <div className="cc-name cc-ltr">{h.remark || '(بدون نام)'}</div>
                    <div className="cc-sub">
                      {h.groupName} · priority {faNum(h.priority)} · #{h.id}
                    </div>
                  </div>
                  {h.disabled && <Badge variant="secondary">خاموش</Badge>}
                  {(h.locked || h.excluded) && <Lock size={15} className="cc-lock" />}
                  <CaretLeft size={14} className="cc-rec-chev" />
                </div>
              )
            })}
            {shown.length === 0 && <Empty title="هاستی پیدا نشد" description="جستجو را عوض کن یا پنل دیگری را انتخاب کن." />}
          </div>
        </>
      )}

      {data && tab === 'prev' && (
        <>
          <div className="cc-hsbar">
            <select className="cc-input" value={tplId} onChange={(e) => setTplId(e.target.value)}>
              {data.templates.map((t) => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
            <select className="cc-input" value={mode} onChange={(e) => setMode(e.target.value)}>
              <option value="both">نام + ترتیب</option>
              <option value="name">فقط نام</option>
              <option value="order">فقط ترتیب</option>
            </select>
            <label className="cc-checkline cc-inline">
              <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} />
              نادیده‌گرفتن استثناها
            </label>
            <Button variant="primary" onClick={preview}>پیش‌نمایش</Button>
            {rows?.some((r) => r.changed) && (
              <Button variant="secondary" onClick={() => setApplyOpen(true)}>اعمال تغییرات</Button>
            )}
          </div>
          {!rows && <div className="cc-hint">قالب را انتخاب کن و پیش‌نمایش بزن؛ هیچ تغییری تا زدن «اعمال» ثبت نمی‌شود.</div>}
          {rows && (
            <div className="cc-trowlist">
              {rows.map((r) => (
                <div key={r.id} className={'cc-diff' + (r.changed ? ' on' : '')}>
                  <div className="cc-diffold cc-ltr">{r.remark || '—'}</div>
                  {r.changed && <div className="cc-diffnew cc-ltr">{r.new}</div>}
                  <div className="cc-sub">
                    {r.groupName} · {faNum(r.priority)} → {faNum(r.newPriority)}
                    {r.locked ? ' · 🔒 قفل' : ''}
                    {r.excluded ? ' · مستثنی' : ''}
                  </div>
                  {(r.warnings || []).map((w) => (
                    <div key={w} className="cc-warn">{w}</div>
                  ))}
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {data && tab === 'exc' && (
        <>
          <div className="cc-hint cc-mb">هاست‌های این لیست در اعمال گروهی دست‌نخورده می‌مانند مگر با تأیید Force.</div>
          {data.overrides.length === 0 && <Empty title="هیچ استثنایی ثبت نشده" description="از تب هاست‌ها روی هر هاست بزن تا قفل یا استثنا بسازی." />}
          <div className="cc-trowlist">
            {data.overrides.map((o) => (
              <ListRow
                key={o.panel + ':' + o.host_id}
                lead={<ShieldWarning size={17} className="cc-lock" />}
                title={'#' + o.host_id}
                meta={
                  ([o.locked_name && 'قفل نام', o.locked_priority && 'قفل ترتیب', o.excluded && 'مستثنی', !o.enabled && 'غیرفعال']
                    .filter(Boolean)
                    .join(' · ') || 'استثنا') + (o.reason ? ' — ' + o.reason : '')
                }
                trailing={
                  <Button variant="ghost" aria-label="حذف استثنا" onClick={() => act('/api/tools/hs/ov', { op: 'del', panel: o.panel, host_id: o.host_id }, () => load())}>
                    <Trash size={15} />
                  </Button>
                }
              />
            ))}
          </div>
        </>
      )}

      {data && tab === 'hist' && (
        <>
          <div className="cc-hsbar">
            <Button variant="outline" onClick={() => act('/api/tools/hs/bk', { op: 'new' }, () => load())}>بکاپ دستی</Button>
          </div>
          <SectionHeader title="بکاپ‌ها" count={faNum(data.backups.length)} />
          <div className="cc-trowlist">
            {data.backups.map((b) => (
              <ListRow
                key={b.id}
                title={faDate(b.ts)}
                meta={faNum(b.count) + ' هاست · ' + b.hash.slice(0, 12) + '…'}
                trailing={
                  <Button variant="ghost" onClick={() => act('/api/tools/hs/bk', { op: 'restore', id: b.id }, () => { load(); setNotice('بازگردانی شد') })}>
                    بازگردانی
                  </Button>
                }
              />
            ))}
            {data.backups.length === 0 && <Empty title="بکاپی نیست" description="قبل از هر اعمال خودکار بکاپ ساخته می‌شود." />}
          </div>
          <SectionHeader title="Audit Log" count={faNum(data.audit.length)} />
          <div className="cc-trowlist">
            {data.audit.map((a, i) => (
              <ListRow
                key={i}
                title={(a.ok ? '✅ اعمال موفق' : '❌ ناموفق') + (a.note ? ' — ' + a.note : '')}
                meta={`${faDate(a.ts)} · کاربر ${faNum(a.uid)} · ${faNum(a.hosts)} هاست · ${a.template}`}
              />
            ))}
            {data.audit.length === 0 && <Empty title="هنوز عملیاتی ثبت نشده" description="Audit Log بعد از اولین اعمال پر می‌شود." />}
          </div>
        </>
      )}

      {busy && <div className="cc-hint">در حال انجام…</div>}

      {edit && (
        <TplEditor
          tpl={edit}
          groups={groups}
          hosts={data?.hosts || []}
          err={editErr}
          onClose={() => { setEdit(null); setEditErr('') }}
          onSave={() => {
            const bad = (edit.segments || []).filter((s) => !s.trim())
            if (bad.length === (edit.segments || []).length) {
              setEditErr('حداقل یک بخش نام لازم است.')
              return
            }
            act('/api/tools/hs/tpl', { op: 'save', template: edit }, () => {
              setEdit(null)
              setEditErr('')
              load()
            })
          }}
        />
      )}

      {hv && ov && (
        <FormDialog
          title={'هاست #' + hv.id}
          sub={hv.remark}
          onClose={() => { setHv(null); setOv(null) }}
          onSave={saveOv}
        >
          <div className="cc-field-label">گروه</div>
          <select
            className="cc-input cc-mb"
            value={hv.group}
            onChange={(e) => {
              const g = e.target.value
              setHv({ ...hv, group: g })
              act('/api/tools/hs/ov', { op: 'group', panel: panel, host_id: hv.id, group: g }, () => load())
            }}
          >
            {groups.map((g) => (
              <option key={g.id} value={g.id}>{g.label}</option>
            ))}
          </select>
          {(data?.overrides || []).some((x) => x.panel === ov.panel && x.host_id === ov.host_id) && (
            <label className="cc-checkline"><input type="checkbox" checked={ov.enabled} onChange={(e) => setOv({ ...ov, enabled: e.target.checked })} /> استثنا فعال باشد</label>
          )}
          <label className="cc-checkline"><input type="checkbox" checked={ov.excluded} onChange={(e) => setOv({ ...ov, excluded: e.target.checked })} /> از اعمال گروهی مستثنی شود</label>
          <label className="cc-checkline"><input type="checkbox" checked={ov.locked_name} onChange={(e) => setOv({ ...ov, locked_name: e.target.checked })} /> قفل نام</label>
          <label className="cc-checkline"><input type="checkbox" checked={ov.locked_priority} onChange={(e) => setOv({ ...ov, locked_priority: e.target.checked })} /> قفل ترتیب</label>
          <div className="cc-field-label">نام دستی (اختیاری)</div>
          <Input className="cc-input cc-mb cc-ltr" value={ov.custom_remark} onChange={(e) => setOv({ ...ov, custom_remark: e.target.value })} placeholder="متن خام remark" />
          <div className="cc-field-label">دلیل</div>
          <Input className="cc-input cc-mb" value={ov.reason} onChange={(e) => setOv({ ...ov, reason: e.target.value })} placeholder="مثلاً کانفیگ مشخصات" />
        </FormDialog>
      )}

      {applyOpen && (
        <Dialog.Root open onOpenChange={(o) => !o && setApplyOpen(false)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">اعمال تغییرات</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              قبل از اعمال یک بکاپ کامل گرفته می‌شود. برای تأیید، کلمهٔ APPLY را تایپ کن.
            </Dialog.Description>
            <div className="cc-form">
              <Input className="cc-input cc-ltr" value={applyWord} onChange={(e) => setApplyWord(e.target.value)} placeholder="APPLY" />
            </div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={() => setApplyOpen(false)}>انصراف</Button>
              <Button variant="primary" disabled={applyWord.trim() !== 'APPLY'} onClick={apply}>اعمال</Button>
            </div>
          </Dialog>
        </Dialog.Root>
      )}
    </div>
  )
}

function blankTpl(d: Data): Tpl {
  const base = d.templates.find((t) => t.is_default) || d.templates[0]
  return {
    id: '',
    name: 'قالب جدید',
    description: '',
    is_default: false,
    scope: 'global',
    segments: base ? [...(base.segments || [])] : ['{flag} {group_icon} {group_name}', '{service}', '{noads}'],
    variables: [],
    fonts: base ? { ...(base.fonts || {}) } : { '0': 'mono', '1': 'bold', '2': 'serif' },
    emojis: base ? { ...(base.emojis || {}) } : {},
    separator: base?.separator || ' | ',
    spacing: base?.spacing || 'spaced',
    group_order: base ? [...base.group_order] : ['direct', 'tunnel', 'fastly', 'cdn', 'unknown', 'status'],
    anchor: { enabled: true, slot: 'first', host_id: '' },
    group_overrides: {},
    host_overrides: {},
    version: 1,
  }
}

function TplEditor({
  tpl,
  groups,
  hosts,
  err,
  onClose,
  onSave,
}: {
  tpl: Tpl
  groups: Group[]
  hosts: HHost[]
  err: string
  onClose: () => void
  onSave: () => void
}) {
  const [t, setT] = useState<Tpl>({ ...tpl, segments: [...(tpl.segments || [])], fonts: { ...(tpl.fonts || {}) }, emojis: { ...(tpl.emojis || {}) }, group_order: [...(tpl.group_order || [])] })
  const set = (p: Partial<Tpl>) => setT({ ...t, ...p })

  function moveGroup(i: number, dir: number) {
    const g = [...t.group_order]
    const j = i + dir
    if (j < 0 || j >= g.length) return
    ;[g[i], g[j]] = [g[j], g[i]]
    set({ group_order: g })
  }

  return (
    <FormDialog title="ویرایش قالب" sub="هر بخش را در یک خط بنویس. متغیرها با کلیک اضافه می‌شوند." onClose={onClose} onSave={onSave} error={err}>
      <div className="cc-field-label">نام قالب</div>
      <Input className="cc-input cc-mb" value={t.name} onChange={(e) => set({ name: e.target.value })} />
      <div className="cc-field-label">توضیح</div>
      <Input className="cc-input cc-mb" value={t.description} onChange={(e) => set({ description: e.target.value })} />

      <div className="cc-field-label">بخش‌های نام</div>
      <textarea
        className="cc-input cc-textarea cc-mb cc-ltr"
        rows={3}
        value={t.segments.join('\n')}
        onChange={(e) => set({ segments: e.target.value.split('\n') })}
      />
      <div className="cc-varchips">
        {VARS.map((v) => (
          <button key={v} type="button" className="cc-chip" onClick={() => set({ segments: [...t.segments.slice(0, -1), (t.segments[t.segments.length - 1] || '') + ' ' + v] })}>
            {v}
          </button>
        ))}
      </div>

      <div className="cc-field-row">
        <div className="cc-grow">
          <div className="cc-field-label">جداکننده</div>
          <Input className="cc-input" value={t.separator} onChange={(e) => set({ separator: e.target.value })} />
        </div>
        <div>
          <div className="cc-field-label">فاصله</div>
          <select className="cc-input" value={t.spacing} onChange={(e) => set({ spacing: e.target.value })}>
            <option value="spaced">فاصله‌دار</option>
            <option value="compact">فشرده</option>
          </select>
        </div>
      </div>

      <div className="cc-field-label">فونت هر بخش</div>
      {t.segments.map((s, i) => (
        <div key={i} className="cc-field-row cc-mb">
          <div className="cc-grow cc-ltr cc-sub">{s || '—'}</div>
          <select
            className="cc-input cc-field-sm"
            value={t.fonts[String(i)] || 'sans'}
            onChange={(e) => set({ fonts: { ...t.fonts, [String(i)]: e.target.value } })}
          >
            {FONTS.map((f) => (
              <option key={f.v} value={f.v}>{f.l}</option>
            ))}
          </select>
        </div>
      ))}

      <div className="cc-field-label">آیکون هر گروه</div>
      {groups.map((g) => (
        <div key={g.id} className="cc-field-row cc-mb">
          <div className="cc-grow">{g.label}</div>
          <Input
            className="cc-input cc-field-sm cc-center"
            value={t.emojis[g.id] || ''}
            onChange={(e) => set({ emojis: { ...t.emojis, [g.id]: e.target.value } })}
          />
        </div>
      ))}

      <div className="cc-field-label">ترتیب گروه‌ها</div>
      {t.group_order.map((gid, i) => {
        const g = groups.find((x) => x.id === gid)
        return (
          <div key={gid} className="cc-orderrow">
            <Check size={14} className="cc-mutedic" />
            <span className="cc-grow">{g?.label || gid}</span>
            <button type="button" className="cc-mini" onClick={() => moveGroup(i, -1)} aria-label="بالا">▲</button>
            <button type="button" className="cc-mini" onClick={() => moveGroup(i, 1)} aria-label="پایین">▼</button>
          </div>
        )
      })}

      <div className="cc-field-label">قفل جایگاه</div>
      <label className="cc-checkline">
        <input type="checkbox" checked={t.anchor.enabled} onChange={(e) => set({ anchor: { ...t.anchor, enabled: e.target.checked } })} />
        قفل کانفیگ ثابت
      </label>
      <div className="cc-field-row">
        <select className="cc-input cc-grow" value={t.anchor.slot} onChange={(e) => set({ anchor: { ...t.anchor, slot: e.target.value } })}>
          <option value="first">کانفیگ اول (priority 0)</option>
          <option value="second">کانفیگ دوم (priority 10)</option>
        </select>
        <select className="cc-input cc-grow" value={String(t.anchor.host_id || '')} onChange={(e) => set({ anchor: { ...t.anchor, host_id: e.target.value } })}>
          <option value="">بدون هاست</option>
          {hosts.map((h) => (
            <option key={h.id} value={h.id}>#{h.id} {h.remark.slice(0, 24)}</option>
          ))}
        </select>
      </div>
      <div className="cc-hint">هاست قفل‌شده نام و ترتیبش تغییر نمی‌کند و در پیش‌نمایش با 🔒 مشخص می‌شود.</div>
    </FormDialog>
  )
}
