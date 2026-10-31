import { CaretLeft, Lightning, Plus } from '@phosphor-icons/react'
import { Button, Empty, Input, LayerCard, Loader } from './ui'
import { locFa } from './hetzner/ui'
import type { OptType, Options } from './hetzner/types'

type CreateCur = { v: 'create'; acc: number; step: number; loc?: string; type?: string; typeName?: string; arch?: string; image?: string; imageName?: string }

export function CreateView(p: {
  cur: CreateCur
  opts: Options | null
  setCur: (v: any) => void
  createName: string
  setCreateName: (v: string) => void
  submitCreate: () => void
  planGroups: (t: OptType[]) => { id: string; title: string; items: OptType[] }[]
  quickCreate: (t: OptType) => void
}) {
    const c = p.cur
    const steps = (
      <div className="cc-steps">
        {[1, 2, 3, 4].map((n) => (
          <i key={n} className={c.step >= n ? 'on' : ''} />
        ))}
      </div>
    )
    if (c.step < 4 && !p.opts) return <>{steps}<div className="cc-center cc-pad"><Loader /></div></>
    if (c.step === 1 && p.opts) {
      return (
        <>
          {steps}
          <div className="cc-section">لوکیشن</div>
          {p.opts.locations.map((l) => (
            <button key={l.name} className="cc-choice" onClick={() => p.setCur({ ...c, step: 2, loc: l.name })}>
              <span className="cc-grow">
                <span className="cc-name">{locFa(l.name, l.city, l.country)}</span>
                <span className="cc-sub cc-ltr">{l.name}</span>
              </span>
            </button>
          ))}
        </>
      )
    }
    if (c.step === 2 && p.opts) {
      return (
        <>
          {steps}
          <div className="cc-section">پلن · {locFa(c.loc || '', '', '')}</div>
          {p.opts.types.length === 0 && <Empty title="پلنی برای این لوکیشن نیست" />}
          {p.planGroups(p.opts.types).map((g) => (
            <div key={g.id} className="cc-plangroup">
              <div className="cc-plangroup-title">{g.title}</div>
              {g.items.map((t) => {
                const custom = () => p.setCur({ ...c, step: 3, type: t.id, typeName: t.name, arch: t.architecture })
                return (
                  <div key={t.id} className="cc-plan">
                    <button className="cc-choice cc-plan-main" onClick={custom}>
                      <span className="cc-grow">
                        <span className="cc-name">{t.name}</span>
                        <span className="cc-sub">
                          {t.cores} vCPU · {t.memory} GB · {t.disk} GB
                        </span>
                      </span>
                      <span className="cc-plan-price">
                        {t.monthly_net ? Number(t.monthly_net).toFixed(2) + ' €' : ''}
                      </span>
                    </button>
                    <div className="cc-plan-acts">
                      <button
                        type="button"
                        className="cc-iconbtn on"
                        title="ساخت فوری (Ubuntu 24.04)"
                        aria-label="ساخت فوری"
                        onClick={() => p.quickCreate(t)}
                      >
                        <Lightning size={16} weight="fill" />
                      </button>
                      <button
                        type="button"
                        className="cc-iconbtn"
                        title="ساخت سفارشی"
                        aria-label="ساخت سفارشی"
                        onClick={custom}
                      >
                        <CaretLeft size={16} weight="bold" />
                      </button>
                    </div>
                  </div>
                )
              })}
            </div>
          ))}
        </>
      )
    }
    if (c.step === 3 && p.opts) {
      return (
        <>
          {steps}
          <div className="cc-section">سیستم‌عامل</div>
          {p.opts.images.map((im) => (
            <button key={im.id} className="cc-choice" onClick={() => p.setCur({ ...c, step: 4, image: im.id, imageName: im.name })}>
              <span className="cc-grow">
                <span className="cc-name">{im.name}</span>
                <span className="cc-sub">
                  {im.architecture} · {im.os_flavor || ''}
                </span>
              </span>
            </button>
          ))}
        </>
      )
    }
    return (
      <>
        {steps}
        <div className="cc-section">نام و تأیید</div>
        <Input
          className="cc-input cc-mb"
          placeholder="نام سرور (مثلاً de-1)"
          value={p.createName}
          maxLength={63}
          onChange={(e) => p.setCreateName(e.target.value)}
        />
        <LayerCard className="cc-card">
          <div className="cc-kv">
            <span>لوکیشن</span>
            <span>{c.loc}</span>
          </div>
          <div className="cc-kv">
            <span>پلن</span>
            <span>{c.typeName || c.type}</span>
          </div>
          <div className="cc-kv">
            <span>تصویر</span>
            <span className="cc-mono">{c.imageName || c.image}</span>
          </div>
          <div className="cc-kv">
            <span>سیستم</span>
            <span>Ubuntu + رمز روت تصادفی</span>
          </div>
        </LayerCard>
        <Button className="cc-wide cc-submit" variant="primary" icon={Plus} onClick={p.submitCreate}>
          ساخت سرور
        </Button>
      </>
    )
}
