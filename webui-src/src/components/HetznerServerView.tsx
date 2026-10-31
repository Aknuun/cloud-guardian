import { ArrowClockwise, ArrowCounterClockwise, ArrowUp, ArrowsClockwise, Camera, GlobeHemisphereWest, Key, PencilSimple, Power, Scissors, Trash, Wrench } from '@phosphor-icons/react'
import { Button, Dialog, Empty, LayerCard, Meter } from './ui'
import { Badge } from './ui'
import { CopyBtn } from './CopyBtn'
import { AppSkeleton } from './AppDialogs'
import { STATUS, locFa, badgeVariant } from './hetzner/ui'
import type { Server, Traffic } from './hetzner/types'
import type { Modal } from './AppDialogs'
import { faDate, faNum, gb, gbf } from '../lib'

export function ServerDetail(p: {
  server: Server | null
  busy: number
  runAction: (op: string, o?: { id?: string; arg?: string; ipId?: number; confirm?: boolean; title?: string; desc?: string }) => void
  doAction: (o: { op: string; id: string; arg?: string; ipId?: number }) => Promise<void>
  setModal: (m: Modal) => void
  toast: (msg: string, bad?: boolean) => void
  resizeFlow: (id: string) => void
  rebuildFlow: (id: string) => void
  ipAssignFlow: (id: string) => void
  copy: (v: string, label?: string) => void
}) {
  const server = p.server
  const busy = p.busy
  const copy = p.copy
      if (!p.server) return p.busy > 0 ? <AppSkeleton rows={3} /> : null
      const s = p.server
      const tr = s.traffic || ({} as Traffic)
      const pct = Math.min(100, Number(tr.used_pct || 0))
      const snaps = s.snapshots || []
      return (
        <>
          <LayerCard className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name">{s.name}</div>
              <div className="cc-sub">
                {s.image || ''} · {s.location ? locFa(s.location.name, s.location.city, s.location.country) : ''} · {s.type ? s.type.id : ''}
              </div>
            </div>
            <Badge variant={badgeVariant(s.status)} dot>
              {STATUS[s.status] || s.status}
            </Badge>
          </LayerCard>

          <LayerCard className="cc-card">
            <div className="cc-kv">
              <span>IPv4</span>
              <span className="cc-mono cc-copyline">
                <span className="cc-copyval">{s.ipv4 || '—'}</span>
                {s.ipv4 && <CopyBtn value={s.ipv4} onCopy={copy} label="کپی IPv4" size={14} />}
              </span>
            </div>
            <div className="cc-kv">
              <span>IPv6</span>
              <span className="cc-mono cc-small cc-copyline">
                <span className="cc-copyval">{s.ipv6 || '—'}</span>
                {s.ipv6 && <CopyBtn value={s.ipv6} onCopy={copy} label="کپی IPv6" size={14} />}
              </span>
            </div>
            <div className="cc-kv">
              <span>پلن</span>
              <span>
                {s.type ? `${s.type.name} · ${s.type.cores} vCPU / ${s.type.memory} GB / ${s.type.disk} GB` : '—'}
              </span>
            </div>
            <div className="cc-kv">
              <span>لوکیشن</span>
              <span>{s.location ? locFa(s.location.name, s.location.city, s.location.country) : '—'}</span>
            </div>
            <div className="cc-kv">
              <span>ساخت</span>
              <span>{faDate(s.created)}</span>
            </div>
            <div className="cc-kv">
              <span>هزینه</span>
              <span>{s.prices && s.prices.monthly ? s.prices.monthly + ' ماهانه' : '—'}</span>
            </div>
          </LayerCard>

          <LayerCard className="cc-card">
            <Meter label="ترافیک مصرفی" value={pct} max={100} showValue={false} />
            <div className="cc-sub cc-mt4">
              {gbf(tr.total_gb)} از {gbf(tr.included_gb)} · {faNum(pct)}٪ مصرف
              {tr.extra_gb > 0 ? ` · ${gbf(tr.extra_gb)} اضافه` : ''}
            </div>
          </LayerCard>

          <div className="cc-actions">
            {s.status === 'running' ? (
              <Button variant="secondary" icon={Power} onClick={() => p.runAction('poweroff', { id: s.id })}>
                خاموش
              </Button>
            ) : (
              <Button variant="primary" icon={Power} onClick={() => p.runAction('poweron', { id: s.id })}>
                روشن
              </Button>
            )}
            <Button variant="secondary" icon={ArrowClockwise} onClick={() => p.runAction('reboot', { id: s.id })}>
              ری‌بوت
            </Button>
            <Button
              variant="secondary"
              icon={ArrowCounterClockwise}
              onClick={() =>
                p.runAction('reset', {
                  id: s.id,
                  confirm: true,
                  title: 'ریست سخت',
                  desc: `عملیات «ریست سخت» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              ریست سخت
            </Button>
            <Button
              variant="secondary"
              icon={Key}
              onClick={() =>
                p.runAction('reset_password', {
                  id: s.id,
                  confirm: true,
                  title: 'ریست رمز روت',
                  desc: `عملیات «ریست رمز روت» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              رمز روت
            </Button>
            <Button
              variant="secondary"
              icon={PencilSimple}
              onClick={() =>
                p.setModal({
                  kind: 'prompt',
                  title: 'نام جدید سرور',
                  placeholder: 'مثلاً de-2',
                  value: s.name,
                  onOk: (v) => {
                    if (!v) {
                      p.toast('نام خالی است', true)
                      return
                    }
                    void p.doAction({ op: 'rename', id: s.id, arg: v })
                  },
                })
              }
            >
              تغییر نام
            </Button>
            <Button
              variant="secondary"
              icon={Camera}
              onClick={() =>
                p.setModal({
                  kind: 'prompt',
                  title: 'توضیح اسنپ‌شات',
                  placeholder: 'اختیاری',
                  onOk: (v) => void p.doAction({ op: 'snapshot_create', id: s.id, arg: v }),
                })
              }
            >
              اسنپ‌شات
            </Button>
            <Button variant="outline" icon={Wrench} onClick={() => void p.rebuildFlow(s.id)}>
              ریبیلد
            </Button>
            <Button variant="outline" icon={ArrowUp} onClick={() => void p.resizeFlow(s.id)}>
              ارتقا
            </Button>
            <Button variant="outline" icon={GlobeHemisphereWest} onClick={() => void p.ipAssignFlow(s.id)}>
              اتصال IP
            </Button>
            <Button
              variant="outline"
              icon={ArrowsClockwise}
              onClick={() =>
                p.runAction('ip_replace', {
                  id: s.id,
                  confirm: true,
                  title: 'جایگزینی IPv4',
                  desc: `سرور «${s.name}» خاموش می‌شود، IPv4 قبلی جدا می‌شود، IPv4 تازه ساخته و متصل می‌شود و بعد سرور روشن می‌شود. ادامه؟`,
                })
              }
            >
              تعویض IPv4
            </Button>
<div className="cc-actions-danger">
            <span className="cc-actions-danger-label">عملیات خطرناک</span>
            <Button
              className="cc-wide"
              variant="secondary-destructive"
              icon={Scissors}
              onClick={() =>
                p.runAction('ip_unassign', {
                  id: s.id,
                  arg: 'ipv4',
                  confirm: true,
                  title: 'حذف IPv4 اختصاصی',
                  desc: `عملیات «حذف IPv4 اختصاصی» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              حذف IPv4 اختصاصی
            </Button>
            <Button
              className="cc-wide"
              variant="secondary-destructive"
              icon={Scissors}
              onClick={() =>
                p.runAction('ip_unassign', {
                  id: s.id,
                  arg: 'ipv6',
                  confirm: true,
                  title: 'حذف IPv6 اختصاصی',
                  desc: `عملیات «حذف IPv6 اختصاصی» روی «${s.name}» انجام شود؟`,
                })
              }
            >
              حذف IPv6 اختصاصی
            </Button>
            <Button
              className="cc-wide"
              variant="destructive"
              icon={Trash}
              onClick={() =>
                p.runAction('delete', {
                  id: s.id,
                  confirm: true,
                  title: 'حذف سرور',
                  desc: `سرور «${s.name}» برای همیشه حذف می‌شود.`,
                })
              }
            >
              حذف سرور
            </Button>
</div>
          </div>

          <div className="cc-section">اسنپ‌شات‌ها ({snaps.length})</div>
          {snaps.length === 0 && <div className="cc-hint cc-center">اسنپ‌شاتی نیست.</div>}
          {snaps.map((sn) => (
            <LayerCard key={sn.id} className="cc-card cc-row">
              <div className="cc-grow">
                <div className="cc-name">{sn.name || sn.id}</div>
                <div className="cc-sub">
                  {gb(sn.size_gb)} · {faDate(sn.created)}
                </div>
              </div>
              <Button
                variant="secondary-destructive"
                size="sm"
                icon={Trash}
                aria-label="حذف اسنپ‌شات"
                onClick={() =>
                  p.setModal({
                    kind: 'confirm',
                    title: 'حذف اسنپ‌شات',
                    desc: 'این اسنپ‌شات برای همیشه حذف می‌شود.',
                    danger: true,
                    onOk: () => p.doAction({ op: 'snapshot_delete', id: s.id, arg: sn.id }),
                  })
                }
              />

            </LayerCard>
          ))}
        </>
      )
}
