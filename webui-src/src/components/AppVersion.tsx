import { useEffect, useState } from 'react'

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
    <span className="cc-ver" dir="ltr" title={version ? `نسخه ${version}` : 'نسخه'}>
      {version ? `v${version}` : 'v…'}
    </span>
  )
}
