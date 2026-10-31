import { useEffect, useState } from 'react'
import { Copy } from '@phosphor-icons/react'
import { Button, Dialog, Input } from './ui'
import { CopyBtn } from './CopyBtn'

export type FormField = {
  key: string
  label: string
  placeholder?: string
  value?: string
  hint?: string
  select?: { value: string; label: string }[]
}

export type ActionModal =
  | { kind: 'confirm'; title: string; desc?: string; danger?: boolean; okText?: string; onOk: () => void | Promise<void> }
  | { kind: 'prompt'; title: string; placeholder?: string; value?: string; onOk: (v: string) => void | Promise<void> }
  | { kind: 'picker'; title: string; items: { id: string; title: string; sub?: string; extra?: string }[]; onPick: (id: string) => void }
  | { kind: 'form'; title: string; desc?: string; fields: FormField[]; okText?: string; onSubmit: (v: Record<string, string>) => void | Promise<void> }
  | { kind: 'password'; title?: string; password: string; note?: string }
  | { kind: 'created'; title?: string; name: string; ip?: string; password: string; onDone?: () => void }

export function ActionModalHost({
  modal,
  onClose,
  onCopy,
}: {
  modal: ActionModal | null
  onClose: () => void
  onCopy: (v: string) => void
}) {
  const [promptVal, setPromptVal] = useState('')
  const [formVal, setFormVal] = useState<Record<string, string>>({})
  useEffect(() => {
    if (modal && modal.kind === 'prompt') setPromptVal(modal.value || '')
    if (modal && modal.kind === 'form') {
      const init: Record<string, string> = {}
      for (const f of modal.fields) init[f.key] = f.value || (f.select && f.select.length ? f.select[0].value : '')
      setFormVal(init)
    }
  }, [modal])
  if (!modal) return null
  return (
    <Dialog.Root open onOpenChange={(o: boolean) => !o && onClose()}>
      <Dialog className="cc-dialog" size="base">
        <Dialog.Close className="cc-dialog-x" aria-label="بستن">
          ✕
        </Dialog.Close>
        {modal.kind === 'confirm' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title}</Dialog.Title>
            {modal.desc && <Dialog.Description className="cc-dialog-desc">{modal.desc}</Dialog.Description>}
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={onClose}>انصراف</Button>
              <Button
                variant={modal.danger ? 'destructive' : 'primary'}
                onClick={() => {
                  const f = modal.onOk
                  onClose()
                  void f()
                }}
              >
                {modal.okText || 'تأیید'}
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'prompt' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title}</Dialog.Title>
            <Input
              className="cc-input cc-mb"
              placeholder={modal.placeholder}
              value={promptVal}
              onChange={(e) => setPromptVal(e.target.value)}
            />
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={onClose}>انصراف</Button>
              <Button
                variant="primary"
                onClick={() => {
                  const f = modal.onOk
                  const v = promptVal.trim()
                  onClose()
                  void f(v)
                }}
              >
                تأیید
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'form' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title}</Dialog.Title>
            {modal.desc && <Dialog.Description className="cc-dialog-desc">{modal.desc}</Dialog.Description>}
            <div className="cc-form">
              {modal.fields.map((f) => (
                <label key={f.key} className="cc-field">
                  <span className="cc-field-label">{f.label}</span>
                  {f.select ? (
                    <select
                      className="cc-input cc-select"
                      value={formVal[f.key] || ''}
                      onChange={(e) => setFormVal((v) => ({ ...v, [f.key]: e.target.value }))}
                    >
                      {f.select.map((o) => (
                        <option key={o.value} value={o.value}>{o.label}</option>
                      ))}
                    </select>
                  ) : (
                    <Input
                      className="cc-input"
                      placeholder={f.placeholder}
                      value={formVal[f.key] || ''}
                      onChange={(e) => setFormVal((v) => ({ ...v, [f.key]: e.target.value }))}
                    />
                  )}
                  {f.hint && <span className="cc-hint">{f.hint}</span>}
                </label>
              ))}
            </div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={onClose}>انصراف</Button>
              <Button
                variant="primary"
                onClick={() => {
                  const f = modal.onSubmit
                  const v = { ...formVal }
                  onClose()
                  void f(v)
                }}
              >
                {modal.okText || 'ثبت'}
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'picker' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title}</Dialog.Title>
            <div className="cc-picker">
              {modal.items.length === 0 && <div className="cc-hint cc-center">گزینه‌ای نیست.</div>}
              {modal.items.map((it) => (
                <button key={it.id} className="cc-choice" onClick={() => modal.onPick(it.id)}>
                  <span className="cc-grow">
                    <span className="cc-name">{it.title}</span>
                    {it.sub && <span className="cc-sub">{it.sub}</span>}
                  </span>
                  {it.extra && <span className="cc-sub">{it.extra}</span>}
                </button>
              ))}
            </div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" onClick={onClose}>انصراف</Button>
            </div>
          </>
        )}
        {modal.kind === 'password' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title || 'رمز روت جدید'}</Dialog.Title>
            {modal.note && <Dialog.Description className="cc-dialog-desc">{modal.note}</Dialog.Description>}
            <div className="cc-secret cc-mono">{modal.password}</div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" icon={Copy} onClick={() => onCopy(modal.password)}>کپی</Button>
              <Button variant="primary" onClick={onClose}>باشه</Button>
            </div>
          </>
        )}
        {modal.kind === 'created' && (
          <>
            <Dialog.Title className="cc-dialog-title">{modal.title || 'سرور ساخته شد'}</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              این اطلاعات را ذخیره کن؛ رمز روت فقط یک‌بار نشان داده می‌شود.
            </Dialog.Description>
            <div className="cc-kv"><span>نام</span><span>{modal.name}</span></div>
            <div className="cc-kv">
              <span>IPv4</span>
              <span className="cc-mono cc-copyline">
                <span className="cc-copyval">{modal.ip || '—'}</span>
                {modal.ip && <CopyBtn value={modal.ip} onCopy={onCopy} label="کپی IPv4" size={14} />}
              </span>
            </div>
            <div className="cc-secret cc-mono">{modal.password}</div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" icon={Copy} onClick={() => onCopy(`${modal.ip} root ${modal.password}`)}>
                کپی همه
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  onClose()
                  modal.onDone && modal.onDone()
                }}
              >
                باشه
              </Button>
            </div>
          </>
        )}
      </Dialog>
    </Dialog.Root>
  )
}
