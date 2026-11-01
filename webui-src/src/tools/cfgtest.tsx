import { useState } from 'react'
import { DownloadSimple, LinkSimple, Pulse, Trash } from '@phosphor-icons/react'
import { api, faNum, timeAgo } from '../lib'
import { Badge, Button, Dialog, Empty } from '../components/ui'
import { ErrBox, Skeleton } from '../components/toolsMeta'
import { SectionHeader } from '../components/SectionHeader'
import { ListRow, SettingsCard } from '../components/ui-kit'
import { useTool } from './shared'

type Pre = { dns_ms: number; tcp_ms: number; tls_ms: number }
type Result = {
  fp: string
  remark: string
  protocol: string
  host: string
  port: number
  verdict: 'ok' | 'degraded' | 'fail'
  pre: Pre
  samples: number[]
  min_ms: number
  median_ms: number
  p95_ms: number
  jitter_ms: number
  loss: number
  error_code?: string
  error_fa?: string
  error_detail?: string
}
type Hist = {
  fp: string
  remark: string
  protocol: string
  host: string
  port: number
  verdict: string
  median_ms: number
  loss: number
  at: number
}
type Data = {
  engine: { ok: boolean; version: string; path: string }
  limits: {
    links: number
    samples: number
    timeout_s: number
    samples_options: number[]
    timeout_options: number[]
  }
  targets: string[]
  history: Hist[]
}

const VERDICT: Record<string, { text: string; variant: 'success' | 'warning' | 'error' }> = {
  ok: { text: 'سالم', variant: 'success' },
  degraded: { text: 'نیمه', variant: 'warning' },
  fail: { text: 'خراب', variant: 'error' },
}

function ms(v: number) {
  return v > 0 ? Math.round(v) + 'ms' : '—'
}

function targetName(t: string) {
  return t.includes('cloudflare') ? 'کلادفلر' : t.includes('gstatic') ? 'گوگل' : t
}

/** Segmented pill control: one track, one active pill. */
function Seg({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: { id: string; label: string }[]
  onChange: (id: string) => void
}) {
  return (
    <fieldset className="cc-cfgopt">
      <legend>{label}</legend>
      <div className="cc-seg2">
        {options.map((o) => (
          <button
            key={o.id}
            type="button"
            className={o.id === value ? 'on' : ''}
            aria-pressed={o.id === value}
            onClick={() => onChange(o.id)}
          >
            {o.label}
          </button>
        ))}
      </div>
    </fieldset>
  )
}

function Bars({ samples, median }: { samples: number[]; median: number }) {
  const top = Math.max(...samples, median, 1)
  // the bar closest to the median carries the accent, so it works even when
  // the median falls between two samples
  const mid = samples.reduce((best, s, i) => {
    if (s <= 0) return best
    if (best < 0 || Math.abs(s - median) < Math.abs(samples[best] - median)) return i
    return best
  }, -1)
  return (
    <div className="cc-bars" role="img" aria-label={'نمونه‌ها: ' + samples.map((s) => (s > 0 ? Math.round(s) + 'ms' : 'ناموفق')).join('، ')}>
      {samples.map((s, i) => (
        <span
          key={i}
          className={'cc-bar' + (s <= 0 ? ' off' : i === mid ? ' on' : '')}
          style={{ height: s > 0 ? Math.max(8, Math.round((s / top) * 100)) + '%' : '18%' }}
          title={s > 0 ? Math.round(s) + 'ms' : 'ناموفق'}
        />
      ))}
    </div>
  )
}

export default function CfgTestView() {
  const { data, err, loaded, reload } = useTool<Data>('/api/tools/cfgtest')
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [runErr, setRunErr] = useState('')
  const [results, setResults] = useState<Result[]>([])
  const [detail, setDetail] = useState<Result | null>(null)
  const [installing, setInstalling] = useState(false)
  const [samples, setSamples] = useState(5)
  const [timeout, setTimeout] = useState(20)
  const [target, setTarget] = useState('')

  if (err) return <ErrBox msg={err} onRetry={reload} />
  if (!loaded || !data) return <Skeleton />

  const links = text
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)

  const run = async () => {
    setRunErr('')
    setBusy(true)
    try {
      const r = await api<{ results: Result[] }>('/api/tools/cfgtest/run', {
        links,
        samples,
        timeout,
        target: target || data.targets[0],
      })
      setResults(r.results || [])
      reload()
    } catch (e: any) {
      setRunErr(e.message || 'تست انجام نشد')
    } finally {
      setBusy(false)
    }
  }

  const install = async () => {
    setInstalling(true)
    setRunErr('')
    try {
      await api('/api/tools/cfgtest/engine', {})
      reload()
    } catch (e: any) {
      setRunErr(e.message || 'نصب هسته انجام نشد')
    } finally {
      setInstalling(false)
    }
  }

  const clearHist = async () => {
    try {
      await api('/api/tools/cfgtest/hist', {})
      reload()
    } catch {
      /* پاک‌کردن تاریخچه حیاتی نیست */
    }
  }

  return (
    <>
      <div className="cc-cfghero">
        <p>لینک یا آدرس ساب را بچسبان؛ اگر کار کند، تأخیر واقعی‌اش را می‌گویم و اگر نه، می‌گویم کجا شکسته.</p>
        <hr className="cc-cfgrule" />
      </div>

      {!data.engine.ok && (
        <SettingsCard
          title="هستهٔ تست نصب نیست"
          footer={
            <Button variant="primary" icon={DownloadSimple} onClick={install} disabled={installing}>
              {installing ? 'در حال نصب…' : 'نصب هستهٔ تست'}
            </Button>
          }
        >
          <div className="cc-hint" style={{ padding: '10px 14px', lineHeight: 1.9 }}>
            برای تست واقعی کانفیگ، هستهٔ xray روی سرور لازم است. یک‌بار دانلود می‌شود و بعد آفلاین کار می‌کند.
          </div>
        </SettingsCard>
      )}

      <div className="cc-cfgcard">
        <div className="cc-cfgfield">
          <textarea
            className="cc-input cc-cfgarea"
            dir="ltr"
            rows={4}
            value={text}
            aria-label="لینک کانفیگ یا آدرس ساب"
            placeholder={'vless://… یا آدرس ساب\nهر خط یک کانفیگ'}
            onChange={(e) => setText(e.target.value)}
          />
          <LinkSimple size={18} className="cc-cfgfield-ico" aria-hidden="true" />
        </div>

        <div className="cc-cfgopts">
          <Seg
            label="نمونه"
            value={String(samples)}
            options={(data.limits.samples_options || [3, 5, 7]).map((n) => ({ id: String(n), label: faNum(n) }))}
            onChange={(id) => setSamples(Number(id))}
          />
          <Seg
            label="مهلت"
            value={String(timeout)}
            options={(data.limits.timeout_options || [10, 20, 30]).map((n) => ({ id: String(n), label: faNum(n) + ' ثانیه' }))}
            onChange={(id) => setTimeout(Number(id))}
          />
          <Seg
            label="مقصد"
            value={target || data.targets[0]}
            options={data.targets.map((t) => ({ id: t, label: targetName(t) }))}
            onChange={setTarget}
          />
        </div>

        {busy ? (
          <div className="cc-cfgpanel" aria-busy="true">
            <div className="cc-cfgshim">
              <span />
              <span />
            </div>
            <div className="cc-preline" style={{ textAlign: 'start' }}>
              در حال تست {faNum(links.length)} کانفیگ · {faNum(samples)} نمونه · {faNum(timeout)} ثانیه مهلت
            </div>
          </div>
        ) : (
          <Button
            variant="primary"
            icon={Pulse}
            className="cc-cfgcta"
            onClick={run}
            disabled={!links.length || !data.engine.ok}
          >
            {links.length > 1 ? 'تست ' + faNum(links.length) + ' کانفیگ' : 'تست کن'}
          </Button>
        )}

        {runErr ? <div className="cc-form-err" style={{ marginTop: 10 }}>{runErr}</div> : null}

        <p className="cc-cfgnote">هیچ لینکی روی سرور ذخیره نمی‌شود؛ فقط نتیجه و یک اثر انگشت کوتاه می‌ماند.</p>

        {links.length > 0 && !busy ? (
          <button type="button" className="cc-cfglinkclear" onClick={() => setText('')}>
            <Trash size={14} /> پاک کردن کادر
          </button>
        ) : null}
      </div>

      {results.length > 0 && <SectionHeader title="نتیجهٔ تست" count={faNum(results.length) + ' کانفیگ'} />}

      <div aria-live="polite">
        {results.length > 0 && (
          <div className="cc-cfglist">
            {results.map((r) => {
              const v = VERDICT[r.verdict] || VERDICT.fail
              return (
                <ListRow
                  key={r.fp + r.host}
                  onClick={() => setDetail(r)}
                  lead={<span className={'cc-st' + (r.verdict === 'ok' ? ' up' : r.verdict === 'fail' ? ' down' : '')} />}
                  title={r.remark || r.host + ':' + r.port}
                  meta={
                    r.protocol +
                    ' · ' +
                    r.host +
                    ':' +
                    r.port +
                    (r.loss > 0 ? ' · ' + faNum(Math.round(r.loss * r.samples.length)) + ' از ' + faNum(r.samples.length) + ' ناموفق' : '')
                  }
                  trailing={
                    <span className="cc-cfgtrail">
                      <b>{ms(r.median_ms)}</b>
                      <Badge variant={v.variant as any} dot>
                        {v.text}
                      </Badge>
                    </span>
                  }
                />
              )
            })}
          </div>
        )}
      </div>

      {!results.length && !busy && (
        <Empty title="هنوز چیزی تست نشده" description="لینک کانفیگ یا آدرس ساب را بالا بچسبان؛ نتیجه همین‌جا می‌آید." />
      )}

      <SectionHeader title="روش اندازه‌گیری" />
      <div className="cc-cfgsteps">
        <div className="cc-cfgstep">
          <span className="cc-cfgnum">۰۱</span>
          <div>
            <b>سه پلهٔ تشخیص</b>
            <span>اول نام دامنه، بعد اتصال TCP و بعد TLS؛ اگر جایی بشکند، همان‌جا را می‌گویم.</span>
          </div>
        </div>
        <div className="cc-cfgstep">
          <span className="cc-cfgnum">۰۲</span>
          <div>
            <b>تأخیر واقعی</b>
            <span>ترافیک از داخل تونل عبور می‌کند و تا اولین بایت پاسخ زمان گرفته می‌شود؛ نه پینگ سادهٔ TCP.</span>
          </div>
        </div>
        <div className="cc-cfgstep">
          <span className="cc-cfgnum">۰۳</span>
          <div>
            <b>چند نمونه، یک عدد</b>
            <span>هر کانفیگ چند بار پشت سر هم تست می‌شود و میانه گزارش می‌شود تا یک کندی لحظه‌ای نتیجه را خراب نکند.</span>
          </div>
        </div>
        <div className="cc-cfgformula">median = median(samples[1..n]) · loss = failed ÷ samples</div>
      </div>

      {data.history.length > 0 && (
        <>
          <SectionHeader
            title="تست‌های قبلی"
            count={faNum(data.history.length) + ' مورد'}
            action={
              <Button variant="ghost" icon={Trash} onClick={clearHist}>
                پاک کردن تاریخچه
              </Button>
            }
          />
          <div className="cc-cfglist">
            {data.history.slice(0, 20).map((h) => {
              const v = VERDICT[h.verdict] || VERDICT.fail
              return (
                <ListRow
                  key={h.fp + h.at}
                  title={h.remark || h.host + ':' + h.port}
                  meta={h.protocol + ' · ' + (timeAgo(h.at * 1000) || 'همین حالا')}
                  trailing={
                    <span className="cc-cfgtrail">
                      <b>{ms(h.median_ms)}</b>
                      <Badge variant={v.variant as any} dot>
                        {v.text}
                      </Badge>
                    </span>
                  }
                />
              )
            })}
          </div>
        </>
      )}

      {detail && (
        <Dialog.Root open onOpenChange={(o) => !o && setDetail(null)}>
          <Dialog className="cc-dialog" size="base">
            <Dialog.Close className="cc-dialog-x">✕</Dialog.Close>
            <Dialog.Title className="cc-dialog-title">{detail.remark || detail.host}</Dialog.Title>
            <Dialog.Description className="cc-dialog-desc">
              {detail.protocol} · {detail.host}:{detail.port}
            </Dialog.Description>
            <div className="cc-bigms" style={{ textAlign: 'start' }}>
              {ms(detail.median_ms)}
              <span className="cc-preline" style={{ marginInlineStart: 8 }}>
                میانه
              </span>
            </div>
            <Bars samples={detail.samples} median={detail.median_ms} />
            <dl className="cc-cfgkv">
              <div>
                <dt>DNS</dt>
                <dd>{ms(detail.pre.dns_ms)}</dd>
              </div>
              <div>
                <dt>اتصال TCP</dt>
                <dd>{ms(detail.pre.tcp_ms)}</dd>
              </div>
              <div>
                <dt>TLS</dt>
                <dd>{ms(detail.pre.tls_ms)}</dd>
              </div>
              <div>
                <dt>کمینه</dt>
                <dd>{ms(detail.min_ms)}</dd>
              </div>
              <div>
                <dt>p95</dt>
                <dd>{ms(detail.p95_ms)}</dd>
              </div>
              <div>
                <dt>فاصلهٔ نمونه‌ها</dt>
                <dd>{ms(detail.jitter_ms)}</dd>
              </div>
              <div>
                <dt>ناموفق</dt>
                <dd>{faNum(Math.round(detail.loss * detail.samples.length))} از {faNum(detail.samples.length)}</dd>
              </div>
            </dl>
            {detail.error_fa ? <div className="cc-errbox" style={{ marginTop: 12 }}>{detail.error_fa}</div> : null}
            {detail.error_detail ? <div className="cc-preline cc-ltr">{detail.error_detail}</div> : null}
          </Dialog>
        </Dialog.Root>
      )}

      <div className="cc-hint cc-center cc-mb">
        سقف هر اجرا {faNum(data.limits.links)} لینک · {faNum(data.limits.samples)} نمونه · {faNum(data.limits.timeout_s)} ثانیه برای هر کانفیگ
      </div>
    </>
  )
}
