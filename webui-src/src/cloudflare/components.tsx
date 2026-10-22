import { useState } from 'react'
import type { ReactNode } from 'react'
import { Badge, Button, Dialog, Input, LayerCard, Switch } from '../components/ui'
import {
  ArrowsClockwise,
  CaretLeft,
  ChartBar,
  Clock,
  Copy,
  Gear,
  Info,
  Link as LinkIcon,
  PencilSimple,
  Star,
  Trash,
  WarningCircle,
  X,
  ArrowClockwise,
} from '@phosphor-icons/react'
import { haptic } from '../lib'
import type { Dlg, Field, RefNode } from './model'
import { isHost, isIPv4, isIPv6, PROXYABLE, relName, TTL_STEPS, ttlLabel, TYPES, validateRecord } from './model'

export function FavStar({
  fav,
  onClick,
  label,
}: {
  fav: boolean
  onClick: () => void
  label?: string
}) {
  return (
    <button
      type="button"
      className={'cc-row-star' + (fav ? ' on' : '')}
      aria-label={label || (fav ? 'حذف از منتخب' : 'افزودن به منتخب')}
      aria-pressed={fav}
      onClick={(e) => {
        e.stopPropagation()
        onClick()
      }}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <Star size={20} weight={fav ? 'fill' : 'regular'} />
    </button>
  )
}

export function SearchInput({
  value,
  onChange,
  placeholder,
  autoFocus,
}: {
  value: string
  onChange: (v: string) => void
  placeholder: string
  autoFocus?: boolean
}) {
  return (
    <div className="cc-search">
      <Input
        autoFocus={autoFocus}
        className="cc-input"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-label={placeholder}
      />
      {value && (
        <Button
          className="cc-search-clear"
          variant="ghost"
          shape="square"
          icon={X}
          aria-label="پاک کردن جست‌وجو"
          onClick={() => onChange('')}
        />
      )}
    </div>
  )
}

export function SkeletonList({ rows = 3 }: { rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, i) => (
        <LayerCard key={i} className="cc-card cc-skel" aria-hidden="true">
          <span className="cc-skel-line" style={{ width: 38 + ((i * 13) % 24) + '%' }} />
          <span className="cc-skel-line" style={{ width: 62 + ((i * 7) % 20) + '%', height: 8 }} />
        </LayerCard>
      ))}
    </>
  )
}

export function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  const analytics = /analytics|آنالیتیکس|Zone → Analytics/i.test(message)
  return (
    <div className="cc-card cc-error-card" role="alert">
      <div className="cc-error-ico">
        <WarningCircle size={24} />
      </div>
      <div className="cc-name">مشکلی پیش آمد</div>
      <div className="cc-sub">{message}</div>
      {analytics && (
        <div className="cc-hintbox" style={{ textAlign: 'right' }}>
          <Info size={17} />
          <span>برای دیدن ترافیک، به توکن کلادفلر دسترسی Zone → Analytics → Read بده و دوباره تلاش کن.</span>
        </div>
      )}
      <Button variant="primary" icon={ArrowClockwise} onClick={onRetry}>
        تلاش دوباره
      </Button>
    </div>
  )
}

export function DialogHead({ title, sub, onClose }: { title: string; sub?: string; onClose: () => void }) {
  return (
    <div className="cc-dialog-head">
      <div className="cc-dialog-headtext">
        <Dialog.Title className="cc-dialog-title">{title}</Dialog.Title>
        {sub && <Dialog.Description className="cc-dialog-sub">{sub}</Dialog.Description>}
      </div>
      <button type="button" className="cc-rec-close" aria-label="بستن" onClick={onClose}>
        <X size={17} />
      </button>
    </div>
  )
}

export function RecordEditor({
  dlg,
  onClose,
  onSubmit,
}: {
  dlg: Extract<Dlg, { k: 'editor' }>
  onClose: () => void
  onSubmit: (v: any) => Promise<void>
}) {
  const r = dlg.rec
  const [type, setType] = useState(r?.type || 'A')
  const [name, setName] = useState(r ? relName(r.name, dlg.zone) : '')
  const [content, setContent] = useState(r?.content || '')
  const [ttl, setTTL] = useState(r?.ttl || 1)
  const [proxied, setProxied] = useState(!!r?.proxied)
  const [priority, setPriority] = useState(r?.priority != null ? String(r.priority) : '')
  const [formErr, setFormErr] = useState('')
  const [busy, setBusy] = useState(false)
  const canProxy = PROXYABLE.includes(type)
  const history = (r?.history || []).filter((v) => v && v !== content)

  async function submit() {
    const problem = validateRecord(type, name, content, priority, !r)
    if (problem) {
      setFormErr(problem)
      haptic('error')
      return
    }
    setFormErr('')
    setBusy(true)
    try {
      await onSubmit({
        op: r ? 'edit' : 'add',
        type,
        name: r ? undefined : name.trim(),
        content: content.trim(),
        ttl: Number(ttl || 1),
        proxied: canProxy && proxied,
        priority: type === 'MX' && priority !== '' ? Number(priority) : undefined,
        force: false,
      })
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <DialogHead
        title={r ? 'ویرایش رکورد' : 'رکورد جدید'}
        sub={r ? `${r.type} · ${r.name}` : `در دامنهٔ ${dlg.zone || 'انتخاب‌شده'}`}
        onClose={onClose}
      />
      {!r ? (
        <>
          <div className="cc-field-label">نوع رکورد</div>
          <div className="cc-type-grid" role="group" aria-label="نوع رکورد">
            {TYPES.map((t) => (
              <button
                key={t}
                type="button"
                className={'cc-type-btn' + (type === t ? ' on' : '')}
                aria-pressed={type === t}
                onClick={() => {
                  setType(t)
                  setFormErr('')
                }}
              >
                {t}
              </button>
            ))}
          </div>
          <div className="cc-field-label">نام رکورد</div>
          <Input
            autoFocus
            className={'cc-input cc-mb' + (formErr && !name.trim() ? ' cc-invalid' : '')}
            placeholder="مثلاً sub یا @"
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              setFormErr('')
            }}
          />
        </>
      ) : (
        <LayerCard className="cc-editor-context">
          <Badge variant="secondary" className={'cc-type cc-type-' + r.type.toLowerCase()}>{r.type}</Badge>
          <div className="cc-grow">
            <div className="cc-name cc-mono">{relName(r.name, dlg.zone)}</div>
            <div className="cc-sub">نام این رکورد قابل تغییر نیست</div>
          </div>
        </LayerCard>
      )}
      <div className="cc-field-label">مقدار</div>
      <Input
        autoFocus={!!r}
        className={'cc-input cc-mb cc-mono' + (formErr ? ' cc-invalid' : '')}
        placeholder={
          type === 'CNAME' ? 'host.example.com' : type === 'MX' ? 'mail.example.com' : type === 'TXT' ? 'متن رکورد' : '1.2.3.4'
        }
        value={content}
        onChange={(e) => {
          setContent(e.target.value)
          setFormErr('')
        }}
      />
      {history.length > 0 && (
        <>
          <div className="cc-field-label">سابقهٔ مقدارهای قبلی</div>
          <div className="cc-history" aria-label="سابقهٔ مقدارهای قبلی">
            {history.map((v) => (
              <button
                key={v}
                type="button"
                className="cc-history-chip cc-mono"
                title="استفاده از این مقدار"
                onClick={() => {
                  setContent(v)
                  setFormErr('')
                }}
              >
                {v}
              </button>
            ))}
          </div>
          <div className="cc-hint cc-mb">برای جایگذاری، روی مقدار قبلی بزن.</div>
        </>
      )}
      {type === 'MX' && (
        <>
          <div className="cc-field-label">اولویت (Priority)</div>
          <Input
            className={'cc-input cc-mb' + (formErr && priority ? ' cc-invalid' : '')}
            placeholder="10"
            value={priority}
            onChange={(e) => {
              setPriority(e.target.value)
              setFormErr('')
            }}
          />
        </>
      )}
      {formErr && <div className="cc-field-err" role="alert">{formErr}</div>}
      <div className="cc-editor-settings">
        <div className="cc-editor-setting">
          <span>
            <span className="cc-editor-setting-title">TTL</span>
            <span className="cc-editor-setting-sub">زمان کش شدن رکورد</span>
          </span>
          <select className="cc-select cc-select-sm" value={ttl} onChange={(e) => setTTL(Number(e.target.value))}>
            {TTL_STEPS.map((t) => (
              <option key={t} value={t}>
                {ttlLabel(t)}
              </option>
            ))}
          </select>
        </div>
        {canProxy && (
          <div className="cc-editor-setting">
            <span>
              <span className="cc-editor-setting-title">پروکسی کلادفلر</span>
              <span className="cc-editor-setting-sub">ترافیک از شبکهٔ کلادفلر عبور کند</span>
            </span>
            <Switch checked={proxied} aria-label="پروکسی کلادفلر" onCheckedChange={setProxied} />
          </div>
        )}
      </div>
      <div className="cc-dialog-actions">
        <Button variant="secondary" size="lg" onClick={onClose}>
          انصراف
        </Button>
        <Button variant="primary" size="lg" disabled={busy} onClick={() => void submit()}>
          {busy ? 'در حال ذخیره…' : r ? 'ذخیره تغییرات' : 'افزودن رکورد'}
        </Button>
      </div>
    </>
  )
}

export function RecordSheet({
  dlg,
  onClose,
  onEdit,
  onCopy,
  onFav,
  onProxy,
  onTTL,
  onType,
  onTraffic,
  onRefs,
  onLB,
  onDelete,
}: {
  dlg: Extract<Dlg, { k: 'record' }>
  onClose: () => void
  onEdit: () => void
  onCopy: (v: string, l?: string) => void
  onFav: () => void
  onProxy: () => void
  onTTL: () => void
  onType: () => void
  onTraffic: () => void
  onRefs: () => void
  onLB: () => void
  onDelete: () => void
}) {
  const r = dlg.rec
  return (
    <>
      <div className="cc-rec-head">
        <div className="cc-rec-headmain">
          <div className="cc-rec-meta">
            <Badge variant="secondary" className={'cc-type cc-type-' + r.type.toLowerCase()}>{r.type}</Badge>
            {PROXYABLE.includes(r.type) && (
              <span className={'cc-rec-proxy' + (r.proxied ? ' on' : '')}>
                {r.proxied ? 'پروکسی روشن' : 'پروکسی خاموش'}
              </span>
            )}
          </div>
          <Dialog.Title className="cc-rec-title">
            {r.label}
            {r.fav && <Star size={15} weight="fill" className="cc-starred" />}
          </Dialog.Title>
          <Dialog.Description className="cc-rec-desc">
            <span className="cc-mono">{r.content}</span>
            <button
              type="button"
              className="cc-rec-copy"
              aria-label="کپی مقدار"
              onClick={() => onCopy(r.content, 'مقدار کپی شد')}
            >
              <Copy size={15} />
            </button>
          </Dialog.Description>
        </div>
        <button type="button" className="cc-rec-close" aria-label="بستن" onClick={onClose}>
          <X size={17} />
        </button>
      </div>

      <div className="cc-rec-toolbar">
        <Button variant="primary" icon={PencilSimple} onClick={onEdit}>
          ویرایش رکورد
        </Button>
        <Button
          variant={r.fav ? 'secondary' : 'outline'}
          className={r.fav ? 'cc-star-on' : ''}
          aria-pressed={r.fav}
          onClick={onFav}
        >
          <Star size={18} weight={r.fav ? 'fill' : 'regular'} />
          {r.fav ? 'منتخب' : 'افزودن به منتخب'}
        </Button>
      </div>

      <div className="cc-kvbox cc-rec-details">
        <div className="cc-kv">
          <span>TTL</span>
          <span>{ttlLabel(r.ttl)}</span>
        </div>
        {PROXYABLE.includes(r.type) && (
          <div className="cc-kv">
            <span>پروکسی کلادفلر</span>
            <Switch checked={r.proxied} onCheckedChange={onProxy} aria-label="پروکسی کلادفلر" />
          </div>
        )}
        <div className="cc-kv">
          <span>نام کامل</span>
          <span className="cc-mono cc-small">{r.name}</span>
        </div>
      </div>

      <div className="cc-section">مدیریت رکورد</div>
      <div className="cc-rec-group">
        <RecRow icon={<Clock size={17} />} title="زمان TTL" sub={ttlLabel(r.ttl)} onClick={onTTL} />
        {(r.type === 'A' || r.type === 'CNAME') && (
          <RecRow
            icon={<ArrowsClockwise size={17} />}
            title="نوع رکورد"
            sub={'تبدیل به ' + (r.type === 'A' ? 'CNAME' : 'A')}
            onClick={onType}
          />
        )}
        <RecRow icon={<ChartBar size={17} />} title="ترافیک امروز" sub="درخواست‌های DNS این رکورد" onClick={onTraffic} />
        <RecRow icon={<LinkIcon size={17} />} title="CNAMEهای وارده" sub="دامنه‌هایی که به این رکورد اشاره می‌کنند" onClick={onRefs} />
        {PROXYABLE.includes(r.type) && (
          <RecRow icon={<Gear size={17} />} title="لود بالانسر و آی‌پی زاپاس" sub="مدیریت گروه و آدرس پشتیبان" onClick={onLB} />
        )}
      </div>

      <button type="button" className="cc-rec-danger" onClick={onDelete}>
        <Trash size={17} />
        <span>حذف رکورد</span>
      </button>
    </>
  )
}

function RecRow({ icon, title, sub, onClick }: { icon: ReactNode; title: string; sub?: string; onClick: () => void }) {
  return (
    <button type="button" className="cc-rec-row" onClick={onClick}>
      <span className="cc-rec-ico">{icon}</span>
      <span className="cc-rec-text">
        <span className="cc-rec-rowtitle">{title}</span>
        {sub && <span className="cc-rec-rowsub">{sub}</span>}
      </span>
      <CaretLeft size={14} weight="bold" className="cc-rec-chev" />
    </button>
  )
}

export function SpareSheet({
  dlg,
  onClose,
  onSet,
  onDel,
}: {
  dlg: Extract<Dlg, { k: 'spare' }>
  onClose: () => void
  onSet: (v: string) => void
  onDel: () => void
}) {
  const [v, setV] = useState(dlg.entry.spare?.content || '')
  const [vErr, setVErr] = useState('')
  function save() {
    const val = v.trim()
    if (!val) {
      setVErr('آی‌پی یا دامنهٔ زاپاس را وارد کن.')
      return
    }
    if (!isIPv4(val) && !isIPv6(val) && !isHost(val)) {
      setVErr('آی‌پی یا میزبان معتبر نیست.')
      haptic('error')
      return
    }
    setVErr('')
    onSet(val)
  }
  return (
    <>
      <DialogHead title="آی‌پی زاپاس" sub={'زاپاس برای ' + dlg.entry.content} onClose={onClose} />
      <div className="cc-hintbox">
        <Info size={17} />
        <span>
          اگر آی‌پی اصلی از کار بیفتد، در صورت پشتیبانی گروه، زاپاس جای آن استفاده می‌شود. «جایگزینی» در صفحهٔ گروه،
          زاپاس را دائمی می‌کند.
        </span>
      </div>
      {dlg.entry.spare && (
        <div className="cc-kv">
          <span>زاپاس فعلی</span>
          <span className="cc-mono">{dlg.entry.spare.content}</span>
        </div>
      )}
      <div className="cc-field-label">آی‌پی یا دامنهٔ زاپاس</div>
      <Input
        autoFocus
        className={'cc-input cc-mb cc-mono' + (vErr ? ' cc-invalid' : '')}
        placeholder="1.2.3.4 یا backup.example.com"
        value={v}
        onChange={(e) => {
          setV(e.target.value)
          setVErr('')
        }}
      />
      {vErr && <div className="cc-field-err" role="alert">{vErr}</div>}
      <div className="cc-dialog-actions">
        {dlg.entry.spare && (
          <Button variant="secondary-destructive" size="lg" icon={Trash} onClick={onDel}>
            حذف زاپاس
          </Button>
        )}
        <Button variant="primary" size="lg" className={dlg.entry.spare ? '' : 'cc-wide'} disabled={!v.trim()} onClick={save}>
          ذخیره
        </Button>
      </div>
    </>
  )
}

export function ConvertRecordDialog({
  dlg,
  onClose,
  onSubmit,
}: {
  dlg: Extract<Dlg, { k: 'convert' }>
  onClose: () => void
  onSubmit: (content: string) => Promise<boolean>
}) {
  const toA = dlg.next === 'A'
  const [value, setValue] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const picked = dlg.candidates.some((c) => c.name === value) ? value : ''

  async function submit() {
    const v = value.trim()
    if (!v) {
      setErr(toA ? 'آی‌پی جدید را وارد کن.' : 'دامنهٔ مقصد را انتخاب یا وارد کن.')
      haptic('error')
      return
    }
    if (toA && !isIPv4(v)) {
      setErr('برای تبدیل به A باید یک آی‌پی معتبر IPv4 وارد کنی.')
      haptic('error')
      return
    }
    if (!toA && !isHost(v)) {
      setErr('برای تبدیل به CNAME باید یک دامنهٔ معتبر وارد کنی.')
      haptic('error')
      return
    }
    setErr('')
    setBusy(true)
    try {
      const ok = await onSubmit(v)
      if (!ok) setErr('تبدیل انجام نشد؛ خطا را بررسی کن.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <DialogHead
        title={toA ? 'تبدیل CNAME به A' : 'تبدیل A به CNAME'}
        sub={`${dlg.rec.type} · ${dlg.rec.name}`}
        onClose={onClose}
      />
      <div className="cc-kvbox">
        <div className="cc-kv">
          <span>مقدار فعلی</span>
          <span className="cc-mono cc-small">{dlg.rec.content}</span>
        </div>
        <div className="cc-kv">
          <span>نوع جدید</span>
          <span>{dlg.next}</span>
        </div>
      </div>
      {!toA && dlg.candidates.length > 0 && (
        <>
          <div className="cc-field-label">انتخاب از رکوردهای موجود</div>
          <select
            autoFocus
            className="cc-select cc-mb"
            value={picked}
            aria-label="انتخاب ساب‌دامنه از رکوردهای موجود"
            onChange={(e) => {
              setValue(e.target.value)
              setErr('')
            }}
          >
            <option value="">انتخاب ساب‌دامنه…</option>
            {dlg.candidates.map((c) => (
              <option key={c.name} value={c.name}>
                {relName(c.name, dlg.zone)} · {c.type}
              </option>
            ))}
          </select>
        </>
      )}
      <div className="cc-field-label">{toA ? 'آی‌پی جدید (IPv4)' : 'یا دامنهٔ مقصد را وارد کن'}</div>
      <Input
        autoFocus={toA || dlg.candidates.length === 0}
        className={'cc-input cc-mb cc-mono' + (err ? ' cc-invalid' : '')}
        placeholder={toA ? '1.2.3.4' : 'target.example.com'}
        value={value}
        onChange={(e) => {
          setValue(e.target.value)
          setErr('')
        }}
      />
      {!toA && <div className="cc-hint cc-mb">می‌توانی از فهرست بالا انتخاب کنی یا دامنه را دستی وارد کنی.</div>}
      {err && <div className="cc-field-err" role="alert">{err}</div>}
      <div className="cc-dialog-actions">
        <Button variant="secondary" size="lg" onClick={onClose}>
          انصراف
        </Button>
        <Button variant="primary" size="lg" disabled={busy} onClick={() => void submit()}>
          {busy ? 'در حال تبدیل…' : 'تبدیل و ذخیره'}
        </Button>
      </div>
    </>
  )
}

function RefTreeNodes({ nodes }: { nodes: RefNode[] }) {
  return (
    <ul className="cc-tree">
      {nodes.map((n, i) => (
        <li className="cc-tree-node" key={n.zone + '|' + n.name + '|' + i}>
          <div className="cc-tree-row">
            <span className="cc-tree-dot" aria-hidden="true" />
            <span className="cc-mono cc-tree-name">{n.name}</span>
            <span className="cc-tree-zone">{n.zone}</span>
            {n.cycle && <span className="cc-tree-cycle">حلقه</span>}
          </div>
          {n.children && n.children.length > 0 && <RefTreeNodes nodes={n.children} />}
        </li>
      ))}
    </ul>
  )
}

export function RefTreeDialog({
  dlg,
  onClose,
}: {
  dlg: Extract<Dlg, { k: 'tree' }>
  onClose: () => void
}) {
  return (
    <>
      <DialogHead
        title={dlg.title}
        sub={dlg.target + (dlg.zone && dlg.zone !== dlg.target ? ' · ' + dlg.zone : '')}
        onClose={onClose}
      />
      {dlg.nodes.length === 0 ? (
        <div className="cc-hint cc-center cc-pad">{dlg.empty || 'هیچ CNAME وارده‌ای پیدا نشد.'}</div>
      ) : (
        <div className="cc-tree-wrap">
          <RefTreeNodes nodes={dlg.nodes} />
        </div>
      )}
      <div className="cc-dialog-actions">
        <Button variant="secondary" onClick={onClose}>
          بستن
        </Button>
      </div>
    </>
  )
}

export function FormDialog({
  fields,
  title,
  desc,
  okText,
  onSubmit,
  onClose,
}: {
  fields: Field[]
  title: string
  desc?: string
  okText?: string
  onSubmit: (v: Record<string, string>) => void | Promise<void>
  onClose: () => void
}) {
  const [vals, setVals] = useState<Record<string, string>>(() => {
    const o: Record<string, string> = {}
    for (const f of fields) o[f.key] = f.value || f.select?.[0]?.value || ''
    return o
  })
  const [errs, setErrs] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  async function go() {
    const next: Record<string, string> = {}
    for (const f of fields) {
      const e = f.validate ? f.validate(vals[f.key] || '') : undefined
      if (e) next[f.key] = e
    }
    if (Object.keys(next).length) {
      setErrs(next)
      haptic('error')
      return
    }
    setErrs({})
    setBusy(true)
    try {
      await onSubmit(vals)
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      <DialogHead title={title} sub={desc} onClose={onClose} />
      {fields.map((f) => (
        <div key={f.key}>
          <div className="cc-field-label">{f.label}</div>
          {f.select ? (
            <select
              className={'cc-select cc-mb' + (errs[f.key] ? ' cc-invalid' : '')}
              value={vals[f.key]}
              aria-invalid={!!errs[f.key]}
              onChange={(e) => {
                setVals({ ...vals, [f.key]: e.target.value })
                setErrs({ ...errs, [f.key]: '' })
              }}
            >
              {f.select.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          ) : (
            <Input
              autoFocus
              type={f.password ? 'password' : 'text'}
              className={'cc-input cc-mb' + (errs[f.key] ? ' cc-invalid' : '')}
              placeholder={f.placeholder}
              aria-invalid={!!errs[f.key]}
              value={vals[f.key]}
              onChange={(e) => {
                setVals({ ...vals, [f.key]: e.target.value })
                setErrs({ ...errs, [f.key]: '' })
              }}
            />
          )}
          {errs[f.key] && (
            <div className="cc-field-err" role="alert">
              {errs[f.key]}
            </div>
          )}
          {f.hint && !errs[f.key] && <div className="cc-hint cc-mb">{f.hint}</div>}
        </div>
      ))}
      <div className="cc-dialog-actions">
        <Button variant="secondary" size="lg" onClick={onClose}>
          انصراف
        </Button>
        <Button variant="primary" size="lg" disabled={busy} onClick={() => void go()}>
          {busy ? 'در حال انجام…' : okText || 'تأیید'}
        </Button>
      </div>
    </>
  )
}

