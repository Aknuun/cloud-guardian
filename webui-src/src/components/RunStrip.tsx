import { timeAgo } from '../lib'

// One line that answers "when did this last run and how did it go".
export function RunStrip({ ts, text, tone }: { ts?: number | string; text?: string; tone?: 'ok' | 'bad' }) {
  if (!ts && !text) return null
  const ms = typeof ts === 'number' ? ts : ts ? Date.parse(ts) : NaN
  const when = Number.isFinite(ms) ? timeAgo(ms) || 'همین حالا' : ts ? String(ts) : ''
  return (
    <div className={'cc-runstrip' + (tone ? ' ' + tone : '')}>
      <span className={'cc-st' + (tone === 'ok' ? ' up' : tone === 'bad' ? ' down' : '')} />
      <span>
        آخرین اجرا: <b>{when}</b>
      </span>
      {text ? <span className="cc-runstrip-txt">{text}</span> : null}
    </div>
  )
}
