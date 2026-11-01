import { useState } from 'react'
import { DownloadSimple, Pulse, Trash } from '@phosphor-icons/react'
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
  limits: { links: number; samples: number; timeout_s: number }
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
    <div className="cc-bars" aria-label={'نمونه‌ها: ' + samples.map((s) => (s > 0 ? Math.round(s) + 'ms' : 'ناموفق')).join('، ')}>
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
      const r = await api<{ results: Result[] }>('/api/tools/cfgtest/run', { links, samples: 5 })
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
      <p className="cc-hint cc-mb">لینک را بچسبان؛ می‌گویم کار می‌کند یا نه و چند میلی‌ثانیه تأخیر دارد.</p>

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

      <SectionHeader
        title="تست زنده"
        count={links.length ? faNum(links.length) + ' لینک' : undefined}
        action={
          links.length ? (
            <Button variant="ghost" icon={Trash} onClick={() => setText('')} aria-label="پاک کردن">
              پاک کردن
            </Button>
          ) : undefined
        }
      />

      <SettingsCard
        footer={
          <>
            <Button
              variant="primary"
              icon={Pulse}
              onClick={run}
              disabled={busy || !links.length || !data.engine.ok}
              aria-busy={busy}
            >
              {busy ? 'در حال تست…' : 'تست کن'}
            </Button>
            {runErr ? <span className="cc-form-err">{runErr}</span> : null}
          </>
        }
      >
        <div style={{ padding: '10px 14px' }}>
          <textarea
            className="cc-input"
            style={{ width: '100%', height: 'auto', minHeight: 92, paddingTop: 10, paddingBottom: 10, lineHeight: 1.9 }}
            dir="ltr"
            rows={4}
            value={text}
            aria-label="لینک کانفیگ"
            placeholder="vless://… یا vmess://… — هر خط یک کانفیگ"
            onChange={(e) => setText(e.target.value)}
          />
        </div>
      </SettingsCard>

      {busy && (
        <div className="cc-runstrip">
          <span className="cc-st" />
          <span>
            در حال تست <b>{faNum(links.length)}</b> کانفیگ…
          </span>
        </div>
      )}

      {results.length > 0 && <SectionHeader title="نتیجهٔ تست" count={faNum(results.length) + ' کانفیگ'} />}

      <div aria-live="polite">
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
                ' · میانه ' +
                ms(r.median_ms) +
                (r.loss > 0 ? ' · ' + Math.round(r.loss * r.samples.length) + ' از ' + r.samples.length + ' ناموفق' : '')
              }
              trailing={
                <Badge variant={v.variant as any} dot>
                  {v.text}
                </Badge>
              }
            />
          )
        })}
      </div>

      {!results.length && !busy && (
        <Empty title="هنوز چیزی تست نشده" description="لینک کانفیگ را بالا بچسبان؛ نتیجه همین‌جا می‌آید." />
      )}

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
          {data.history.slice(0, 20).map((h) => {
            const v = VERDICT[h.verdict] || VERDICT.fail
            return (
              <ListRow
                key={h.fp + h.at}
                title={h.remark || h.host + ':' + h.port}
                meta={h.protocol + ' · ' + ms(h.median_ms) + ' · ' + (timeAgo(h.at * 1000) || 'همین حالا')}
                trailing={
                  <Badge variant={v.variant as any} dot>
                    {v.text}
                  </Badge>
                }
              />
            )
          })}
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
            <div className="cc-bigms">{ms(detail.median_ms)}</div>
            <div className="cc-preline">
              DNS {ms(detail.pre.dns_ms)} · TCP {ms(detail.pre.tcp_ms)} · TLS {ms(detail.pre.tls_ms)}
            </div>
            <Bars samples={detail.samples} median={detail.median_ms} />
            <div className="cc-preline">
              کمینه {ms(detail.min_ms)} · p95 {ms(detail.p95_ms)} · فاصلهٔ نمونه‌ها {ms(detail.jitter_ms)}
            </div>
            {detail.error_fa ? <div className="cc-errbox" style={{ marginTop: 12 }}>{detail.error_fa}</div> : null}
          </Dialog>
        </Dialog.Root>
      )}

      <div className="cc-hint cc-center cc-mb">سقف هر اجرا {faNum(data.limits.links)} لینک · پنج نمونه · {faNum(data.limits.timeout_s)} ثانیه برای هر کانفیگ</div>
    </>
  )
}
