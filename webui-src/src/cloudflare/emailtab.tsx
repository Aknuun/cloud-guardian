import { EnvelopeSimple, Plus, Trash } from '@phosphor-icons/react'
import { Badge, Button, Empty, LayerCard } from '../components/ui'
import { SkeletonList } from './components'
import { isEmail } from './model'
import type { EmailData } from './model'

type View = { v: string; acc?: number; zid?: string }

export function EmailTab({ view, email, num, setDlg, post, openCatchAll }: {
  view: View
  email: EmailData | null
  num: (n: number | undefined) => string
  setDlg: (d: any) => void
  post: (path: string, body: any, keep?: boolean, opts?: { refresh?: boolean }) => Promise<any>
  openCatchAll: () => void
}) {

    if (!view.zid!) return null
    if (!email) return <SkeletonList rows={2} />
    if (!email.available) {
      return (
        <Empty
          title="دسترسی ایمیل نیست"
          description="به توکن دسترسی Zone → Email Routing Rules → Edit اضافه کن."
        />
      )
    }
    if (!email.enabled) {
      return (
        <>
          <LayerCard className="cc-card">
            <div className="cc-name">ایمیل فوروارد</div>
            <div className="cc-sub">
              با فعال‌سازی، رکوردهای MX/SPF خودکار ساخته می‌شوند. اگر سرویس ایمیل دیگری داری، خراب می‌شود.
            </div>
          </LayerCard>
          <Button
            className="cc-wide cc-green"
            icon={EnvelopeSimple}
            onClick={() =>
              setDlg({
                k: 'confirm',
                title: 'فعال‌سازی ایمیل',
                desc: 'ایمیل روی این دامنه فعال شود؟',
                okText: 'فعال کن',
                onOk: async () => {
                  if (!!view.zid!) await post('/api/cf/email', { op: 'enable', acc: view.acc!, zid: view.zid! })
                },
              })
            }
          >
            فعال‌سازی ایمیل
          </Button>
        </>
      )
    }
    const verified = (email.dests || []).filter((d) => d.verified)
    return (
      <>
        <div className="cc-section">قوانین فوروارد ({num(email.rules.length)})</div>
        {email.rules.length === 0 && <div className="cc-hint cc-center cc-mb">قانونی نیست.</div>}
        {email.rules.map((r) => (
          <LayerCard key={r.id} className="cc-card cc-row">
            <div className="cc-grow">
              <div className="cc-name cc-mono">{r.label}</div>
              <div className="cc-sub cc-mono">→ {r.dest}</div>
            </div>
            <Button
              variant="secondary-destructive"
              size="sm"
              icon={Trash}
              aria-label="حذف قانون"
              onClick={() =>
                setDlg({
                  k: 'confirm',
                  title: 'حذف قانون',
                  desc: `${r.label} → ${r.dest} حذف شود؟`,
                  danger: true,
                  okText: 'حذف',
                  onOk: async () => {
                    if (!!view.zid!) await post('/api/cf/email', { op: 'del', acc: view.acc!, zid: view.zid!, rid: r.id })
                  },
                })
              }
            />

          </LayerCard>
        ))}
        <Button
          className="cc-wide cc-green cc-mb"
          icon={Plus}
          onClick={() =>
            setDlg({
              k: 'form',
              title: 'آدرس جدید',
              desc: 'آدرس محلی (مثلاً info) و مقصد تأییدشده را وارد کن.',
              fields: [
                {
                  key: 'addr',
                  label: 'آدرس',
                  placeholder: 'info',
                  hint: 'فقط بخش قبل از @ را وارد کن.',
                  validate: (v: string) =>
                    /^[a-z0-9._%+-]+$/i.test(v.trim()) ? undefined : 'آدرس محلی معتبر نیست.',
                },
                {
                  key: 'dest',
                  label: 'مقصد',
                  select: verified.length
                    ? verified.map((d) => ({ value: d.email, label: d.email }))
                    : undefined,
                  placeholder: 'you@gmail.com',
                  validate: (v: string) => (isEmail(v.trim()) ? undefined : 'ایمیل مقصد معتبر نیست.'),
                },
              ],
              okText: 'ساختن',
              onSubmit: async (v: any) => {
                if (!!view.zid!)
                  await post('/api/cf/email', { op: 'add', acc: view.acc!, zid: view.zid!, addr: v.addr, dest: v.dest })
              },
            })
          }
        >
          آدرس فوروارد
        </Button>

        <div className="cc-section">catch-all</div>
        <LayerCard className="cc-card cc-row">
          <div className="cc-grow">
            <div className="cc-sub">ایمیل‌های بدون قانون</div>
            <div className="cc-name">{email.catchAll || '—'}</div>
          </div>
          <Button variant="outline" size="sm" onClick={openCatchAll}>
            تنظیم
          </Button>
        </LayerCard>

        <div className="cc-section">مقصدها ({num((email.dests || []).length)})</div>
        {(email.dests || []).map((d) => (
          <LayerCard key={d.email} className="cc-card cc-row">
            <span className={'cc-dot ' + (d.verified ? 'run' : 'wait')} />
            <div className="cc-grow">
              <div className="cc-name cc-mono">{d.email}</div>
              <div className="cc-sub">{d.verified ? 'تأییدشده' : 'در انتظار تأیید'}</div>
            </div>
          </LayerCard>
        ))}
        <Button
          className="cc-wide cc-mt"
          variant="outline"
          icon={Plus}
          onClick={() =>
            setDlg({
              k: 'form',
              title: 'افزودن مقصد',
              desc: 'لینک تأیید به این ایمیل می‌رود.',
              fields: [
                {
                  key: 'dest',
                  label: 'ایمیل مقصد',
                  placeholder: 'you@gmail.com',
                  validate: (v: string) => (isEmail(v.trim()) ? undefined : 'ایمیل معتبر نیست.'),
                },
              ],
              okText: 'افزودن',
              onSubmit: async (v: any) => {
                if (!!view.zid!)
                  await post('/api/cf/email', { op: 'dest_add', acc: view.acc!, zid: view.zid!, dest: v.dest })
              },
            })
          }
        >
          مقصد جدید
        </Button>
      </>
    )
  
}
