import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { api, invalidateCache } from '../lib'
import { Button, Dialog } from '../components/ui'

export type Srv = { id: string; name: string; host: string; user: string; port: number; note: string; key: boolean; secret?: string; last?: { ts?: number; ok?: boolean; err?: string } }
export type Rem = { id: string; text: string; when: number; notified: boolean; ack: boolean; server: string; note?: string }
export type PgPanel = { idx: number; name: string; url: string; user: string }
export type PgMon = { id: string; name: string; panel: string; interval: number; enabled: boolean }
export type PgData = { panels: PgPanel[]; monitors: PgMon[] }

export function useTool<T>(path: string) {
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
export async function save(path: string, body: any, done: () => void) {
  await api(path, body)
  invalidateCache()
  done()
}

export const faTxt = (v: string) => (v || '').replace(/[0-9]/g, (d) => '۰۱۲۳۴۵۶۷۸۹'[Number(d)])

export function FormDialog({
  title,
  sub,
  onClose,
  onSave,
  children,
  error,
  disabled,
}: {
  title: string
  sub?: string
  onClose: () => void
  onSave: () => void
  children: ReactNode
  error?: string
  disabled?: boolean
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
          <Button variant="primary" onClick={onSave} disabled={disabled}>
            ذخیره
          </Button>
        </div>
      </Dialog>
    </Dialog.Root>
  )
}

export function verGt(a: string, b: string) {
  const pa = a.replace(/^v/, '').split('.').map(Number)
  const pb = b.replace(/^v/, '').split('.').map(Number)
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const x = pa[i] || 0
    const y = pb[i] || 0
    if (x !== y) return x > y
  }
  return false
}
