import { useEffect, useState } from 'react'
import { Browsers } from '@phosphor-icons/react'

let cachedVersion = ''
let versionRequest: Promise<string> | null = null

function fetchVersion(): Promise<string> {
  if (cachedVersion) return Promise.resolve(cachedVersion)
  if (!versionRequest) {
    versionRequest = fetch('/health', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        const v = typeof d?.version === 'string' ? d.version.trim() : ''
        if (v) cachedVersion = v
        return cachedVersion
      })
      .catch(() => '')
      .finally(() => {
        versionRequest = null
      })
  }
  return versionRequest
}

export function AppVersion() {
  const [version, setVersion] = useState(cachedVersion)

  useEffect(() => {
    let alive = true
    fetchVersion().then((v) => {
      if (alive && v) setVersion(v)
    })
    return () => {
      alive = false
    }
  }, [])

  return (
    <>
      <button
        type="button"
        className="cc-ver cc-chan"
        title="باز کردن نسخه وب در مرورگر"
        onClick={() => {
          const url = location.origin + '/app/'
          const tg = (window as any).Telegram?.WebApp
          if (tg?.openLink) tg.openLink(url)
          else window.open(url, '_blank', 'noopener')
        }}
      >
        <Browsers size={14} />
        <span>نسخه وب</span>
      </button>
      {version && (
        <span className="cc-ver" dir="ltr" title={`نسخه ${version}`}>
          v{version}
        </span>
      )}
    </>
  )
}
