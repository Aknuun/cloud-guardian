import { useEffect, useState } from 'react'
import { TelegramLogo } from '@phosphor-icons/react'

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

export function useVersion() {
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
  return version
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
      <a className="cc-ver cc-chan" href="https://t.me/openWEBcode" target="_blank" rel="noreferrer" title="کانال تلگرام">
        <TelegramLogo size={14} />
        <span dir="ltr">@openWEBcode</span>
      </a>
      {version && (
        <span className="cc-ver" dir="ltr" title={`نسخه ${version}`}>
          v{version}
        </span>
      )}
    </>
  )
}
