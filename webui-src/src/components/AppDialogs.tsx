import { useEffect, useState } from 'react'
import { Copy } from '@phosphor-icons/react'
import { Button, Dialog, Input, LayerCard } from './ui'
import { CopyBtn } from './CopyBtn'

export type PickerItem = { id: string; title: string; sub?: string; extra?: string }
export type Modal =
  | { kind: 'confirm'; title: string; desc?: string; danger?: boolean; okText?: string; onOk: () => void | Promise<void> }
  | { kind: 'prompt'; title: string; placeholder?: string; value?: string; onOk: (v: string) => void | Promise<void> }
  | { kind: 'picker'; title: string; items: PickerItem[]; onPick: (id: string) => void }
  | { kind: 'password'; password: string }
  | { kind: 'created'; name: string; ip: string; password: string; onDone: () => void }

export function ModalHost({ modal, onClose, onCopy }: { modal: Modal | null; onClose: () => void; onCopy: (v: string) => void }) {
  const [promptVal, setPromptVal] = useState('')
  useEffect(() => {
    if (modal && modal.kind === 'prompt') setPromptVal(modal.value || '')
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
              <Button variant="secondary" onClick={onClose}>
                انصراف
              </Button>
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
              <Button variant="secondary" onClick={onClose}>
                انصراف
              </Button>
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
              <Button variant="secondary" onClick={onClose}>
                انصراف
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'password' && (
          <>
            <Dialog.Title className="cc-dialog-title">رمز روت جدید</Dialog.Title>
            <div className="cc-secret cc-mono">{modal.password}</div>
            <div className="cc-dialog-actions">
              <Button variant="secondary" icon={Copy} onClick={() => onCopy(modal.password)}>
                کپی
              </Button>
              <Button variant="primary" onClick={onClose}>
                باشه
              </Button>
            </div>
          </>
        )}
        {modal.kind === 'created' && (
          <>
            <Dialog.Title className="cc-dialog-title">سرور ساخته شد</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              این اطلاعات را ذخیره کن؛ رمز روت فقط یک‌بار نشان داده می‌شود.
            </Dialog.Description>
            <div className="cc-kv">
              <span>نام</span>
              <span>{modal.name}</span>
            </div>
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
                  const f = modal.onDone
                  onClose()
                  f()
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


export function AppSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, i) => (
        <LayerCard key={i} className="cc-card cc-skel" aria-hidden="true">
          <span className="cc-skel-line" style={{ width: 40 + ((i * 11) % 24) + '%' }} />
          <span className="cc-skel-line" style={{ width: 64 + ((i * 9) % 18) + '%', height: 8 }} />
        </LayerCard>
      ))}
    </>
  )
}
