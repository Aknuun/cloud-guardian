import { ArrowClockwise, ArrowRight, Plus, Trash } from '@phosphor-icons/react'
import { Badge, Button, Dialog, Empty, LayerCard } from './ui'
import { CopyBtn } from './CopyBtn'
import { AppSkeleton } from './AppDialogs'
import { locFa } from './hetzner/ui'
import type { CleanIP } from './hetzner/types'

type IpForm = { loc: string; count: number; locs: { name: string; city?: string; country?: string }[] }

export function IpsView(p: {
  ips: CleanIP[]
  busy: number
  ipBusy: boolean
  ipForm: IpForm | null
  setIpForm: (v: any) => void
  submitIpAdd: () => void
  openIpAdd: () => void
  ipFreeFlow: (id: any) => void
  runAction: (op: string, o: any) => void
  copy: (v: string, l?: string) => void
  setCur: (v: any) => void
  acc: number
}) {

      return (
        <>
          <div className="cc-btnrow cc-mb">
            <Button variant="primary" icon={Plus} onClick={() => void p.openIpAdd()} disabled={p.ipBusy}>
              افزودن آی‌پی
            </Button>
            <Button variant="outline" icon={ArrowRight} onClick={() => p.setCur({ v: 'servers', acc: p.acc })}>
              سرورها
            </Button>
            <Button variant="secondary" icon={ArrowClockwise} onClick={() => p.setCur((c: any) => ({ ...c }))}>
              بروزرسانی
            </Button>
          </div>
          {p.ipForm && (
            <Dialog.Root open onOpenChange={(o: boolean) => !o && p.setIpForm(null)}>
              <Dialog className="cc-dialog" size="base">
                <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
                <Dialog.Title className="cc-dialog-title">افزودن آی‌پی تمیز</Dialog.Title>
                <Dialog.Description className="cc-dialog-desc">
                  آی‌پی IPv4 تازه در حساب هتزنر ساخته می‌شود؛ بعد از ساخت می‌توانی وضعیت تمیزی‌اش را بررسی کنی.
                </Dialog.Description>
                <label className="cc-field">
                  <span>لوکیشن</span>
                  <select
                    className="cc-input"
                    value={p.ipForm.loc}
                    onChange={(e) => p.setIpForm({ ...p.ipForm, loc: e.target.value })}
                  >
                    {p.ipForm.locs.map((l) => (
                      <option key={l.name} value={l.name}>
                        {locFa(l.name, l.city, l.country)}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="cc-field">
                  <span>تعداد</span>
                  <input
                    className="cc-input"
                    type="number"
                    min={1}
                    max={10}
                    value={p.ipForm.count}
                    onChange={(e) => p.setIpForm({ ...p.ipForm, count: Math.max(1, Math.min(10, Number(e.target.value) || 1)) })}
                  />
                </label>
                <div className="cc-dialog-actions">
                  <Button variant="secondary" onClick={() => p.setIpForm(null)}>
                    انصراف
                  </Button>
                  <Button variant="primary" onClick={() => void p.submitIpAdd()} disabled={p.ipBusy}>
                    بساز
                  </Button>
                </div>
              </Dialog>
            </Dialog.Root>
          )}
          {p.ips.length === 0 && p.busy > 0 && <AppSkeleton />}
          {p.ips.length === 0 && p.busy === 0 && <Empty title="آی‌پی‌ای در استخر نیست" />}
          {p.ips.map((ip) => {
            const c = ip.check
            const badge = !c ? (
              <Badge variant="secondary">بررسی نشده</Badge>
            ) : c.clean ? (
              <Badge variant="success">تمیز</Badge>
            ) : c.listed && c.listed.length ? (
              <Badge variant="error">لیست‌شده</Badge>
            ) : (
              <Badge variant="warning">نامشخص</Badge>
            )
            const busyIp = !!ip.assignee
            return (
              <LayerCard key={ip.id} className="cc-card cc-row">
                <div className="cc-grow">
                  <div className="cc-name cc-mono cc-copyline">
                    <span>{ip.ip}</span>
                    <CopyBtn value={ip.ip} onCopy={p.copy} label="کپی آی‌پی" />
                  </div>
                  <div className="cc-sub">
                    {ip.loc || ''}
                    {busyIp ? ' · ' + ip.assignee : ' · آزاد'}
                    {c && c.listed && c.listed.length ? ' · ' + c.listed.slice(0, 2).join(', ') : ''}
                  </div>
                </div>
                {badge}
                <div className="cc-row-actions">
                  {!busyIp && (
                    <Button variant="outline" size="sm" onClick={() => void p.ipFreeFlow(ip.id)}>
                      اتصال
                    </Button>
                  )}
                  <Button
                    variant="secondary-destructive"
                    size="sm"
                    icon={Trash}
                    aria-label="حذف آی‌پی"
                    onClick={() =>
                      p.runAction('ip_delete', {
                        id: String(ip.id),
                        ipId: ip.id,
                        confirm: true,
                        title: 'حذف آی‌پی',
                        desc: busyIp
                          ? `آی‌پی «${ip.ip}» به «${ip.assignee}» وصل است. اول جدا می‌شود، بعد حذف — ادامه؟`
                          : `آی‌پی «${ip.ip}» از حساب هتزنر حذف شود؟ این عمل بازگشت‌پذیر نیست.`,
                      })
                    }
                  />
                </div>
              </LayerCard>
            )
          })}
        </>
      )
}
