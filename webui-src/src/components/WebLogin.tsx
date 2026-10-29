import { useEffect, useState } from 'react'
import { ArrowClockwise, TelegramLogo } from '@phosphor-icons/react'
import { setToken } from '../lib'
import { Button, Loader } from './ui'

type Status = 'starting' | 'pending' | 'expired' | 'denied' | 'error'

export function WebLogin({ onDone }: { onDone: () => void }) {
  const [code, setCode] = useState('')
  const [status, setStatus] = useState<Status>('starting')
  const [err, setErr] = useState('')
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    let stop = false
    setStatus('starting')
    setErr('')
    fetch('/api/login/start', { method: 'POST' })
      .then(async (res) => {
        const j = await res.json()
        if (!res.ok || !j.ok) throw new Error(j.error || 'شروع ورود ناموفق بود')
        if (!stop) {
          setCode(j.data.code)
          setStatus('pending')
        }
      })
      .catch((e: any) => {
        if (!stop) {
          setErr(e.message || 'شروع ورود ناموفق بود')
          setStatus('error')
        }
      })
    return () => {
      stop = true
    }
  }, [nonce])

  useEffect(() => {
    if (status !== 'pending' || !code) return
    const id = setInterval(async () => {
      try {
        const res = await fetch('/api/login/state?c=' + encodeURIComponent(code))
        const j = await res.json()
        if (!res.ok || !j.ok) return
        if (j.data.status === 'ok') {
          clearInterval(id)
          setToken(j.data.token)
          onDone()
        } else if (j.data.status === 'expired') {
          clearInterval(id)
          setStatus('expired')
        } else if (j.data.status === 'denied') {
          clearInterval(id)
          setStatus('denied')
        }
      } catch {
        /* keep polling */
      }
    }, 2000)
    return () => clearInterval(id)
  }, [status, code, onDone])

  const shown = code ? code.slice(0, 6).toUpperCase() : ''

  return (
    <div className="cc-main cc-signout">
      <div className="cc-card cc-login-card" role="status">
        <div className="cc-error-ico">
          <TelegramLogo size={24} />
        </div>
        {status === 'pending' && (
          <>
            <div className="cc-name">تأیید ورود در تلگرام</div>
            <div className="cc-sub">درخواست به ربات ارسال شد. برای ورود، تأیید کن.</div>
            <div className="cc-login-code" aria-label="کد ورود">
              <span>{shown.slice(0, 3)}</span>
              <span>{shown.slice(3, 6)}</span>
            </div>
            <div className="cc-login-wait">
              <Loader size="sm" />
              <span>منتظر تأیید…</span>
            </div>
          </>
        )}
        {status === 'starting' && <Loader size="lg" />}
        {(status === 'expired' || status === 'denied' || status === 'error') && (
          <>
            <div className="cc-name">
              {status === 'expired' ? 'کد منقضی شد' : status === 'denied' ? 'درخواست رد شد' : 'ورود ناموفق'}
            </div>
            <div className="cc-sub">{status === 'error' ? err : 'برای دریافت کد تازه، دوباره تلاش کن.'}</div>
            <Button variant="primary" icon={ArrowClockwise} onClick={() => setNonce((n) => n + 1)}>
              تلاش دوباره
            </Button>
          </>
        )}
      </div>
    </div>
  )
}
