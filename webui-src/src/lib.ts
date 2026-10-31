export const TG: any = (window as any).Telegram && (window as any).Telegram.WebApp ? (window as any).Telegram.WebApp : null

export let token = sessionStorage.getItem('cg_token') || ''

export function setToken(v: string) {
  token = v
  if (v) sessionStorage.setItem('cg_token', v)
  else sessionStorage.removeItem('cg_token')
  invalidateCache()
}

// Short-lived cache so revisiting a page paints instantly instead of
// refetching every list. Any write clears it.
const CACHE_TTL = 60_000
const cache = new Map<string, { t: number; v: unknown }>()
const inflight = new Map<string, Promise<unknown>>()

export function invalidateCache() {
  cache.clear()
  inflight.clear()
  lastHit = 0
}

// When data on screen came from the short cache, UIs can say how old it is.
let lastHit = 0
export function cacheHitAge(): number {
  return lastHit ? Date.now() - lastHit : 0
}

export class ApiError extends Error {}

export async function api<T = any>(path: string, body?: any, signal?: AbortSignal): Promise<T> {
  const isGet = body === undefined
  if (isGet) {
    const hit = cache.get(path)
    if (hit && Date.now() - hit.t < CACHE_TTL) {
      lastHit = hit.t
      return hit.v as T
    }
    const running = inflight.get(path)
    if (running && !signal) return running as Promise<T>
  } else {
    invalidateCache()
  }
  const run = request<T>(path, body, signal)
  if (isGet) {
    inflight.set(path, run)
    run
      .then((v) => cache.set(path, { t: Date.now(), v }))
      .catch(() => {})
      .finally(() => inflight.delete(path))
  }
  return run
}

// Same payload + same path sent again within a few seconds = the same logical
// action (double click, impatient retry). Reuse the key so the server replays
// the first result instead of running the mutation twice.
const idemKeys = new Map<string, { k: string; t: number }>()

function idemKey(path: string, body: any): string {
  const sig = path + '|' + JSON.stringify(body)
  const now = Date.now()
  const hit = idemKeys.get(sig)
  if (hit && now - hit.t < 8000) return hit.k
  for (const [s, v] of idemKeys) if (now - v.t > 30000) idemKeys.delete(s)
  const k = (crypto as any).randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2) + now
  idemKeys.set(sig, { k, t: now })
  return k
}

async function request<T>(path: string, body?: any, signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { Authorization: 'Bearer ' + token }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    headers['Idempotency-Key'] = idemKey(path, body)
  }
  const res = await fetch(path, {
    method: body === undefined ? 'GET' : 'POST',
    headers,
    signal,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  let j: any = null
  try {
    j = await res.json()
  } catch {
    /* ignore */
  }
  if (res.status === 401) setToken('')
  if (!res.ok || !j || j.ok === false) throw new ApiError((j && j.error) || 'خطای سرور (' + res.status + ')')
  return j.data as T
}

export function num(v: any) {
  return Number(v || 0).toLocaleString('en-US')
}
export function faNum(v: any) {
  return Number(v || 0).toLocaleString('fa-IR', { maximumFractionDigits: 1 })
}
export function gb(v: any) {
  const n = Number(v || 0)
  return n >= 1000 ? (n / 1000).toFixed(1) + ' TB' : n.toFixed(n < 10 ? 1 : 0) + ' GB'
}
export function gbf(v: any) {
  const n = Number(v || 0)
  return n >= 1000 ? faNum(n / 1000) + ' ترابایت' : faNum(n) + ' گیگابایت'
}
export function mem(mb: any) {
  const n = Number(mb || 0)
  return n >= 1024 ? (n / 1024).toFixed(n % 1024 ? 1 : 0) + ' GB' : n + ' MB'
}
export async function fallbackCopy(v: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(v)
      return true
    }
  } catch {
    /* fall through to execCommand */
  }
  const ta = document.createElement('textarea')
  ta.value = v
  ta.style.position = 'fixed'
  ta.style.top = '0'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.focus()
  ta.select()
  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch {
    ok = false
  }
  ta.remove()
  return ok
}

export function haptic(kind: 'light' | 'success' | 'error' = 'light') {
  try {
    if (!TG || !TG.HapticFeedback) return
    if (kind === 'light') TG.HapticFeedback.impactOccurred('light')
    else TG.HapticFeedback.notificationOccurred(kind)
  } catch {
    /* ignore */
  }
}

export function faDate(v: any) {
  if (!v) return '—'
  const d = new Date(typeof v === 'number' ? v : String(v))
  if (Number.isNaN(d.getTime())) return String(v).slice(0, 10)
  try {
    return new Intl.DateTimeFormat('fa-IR', { dateStyle: 'medium' }).format(d)
  } catch {
    return d.toLocaleDateString('fa-IR')
  }
}

// DNS/IP sanity check used before saving a record anywhere in the panel.
export async function checkRecord(
  type: string,
  value: string,
): Promise<{ ok: boolean; message: string }> {
  const t = (type || '').toUpperCase().trim()
  const v = (value || '').trim()
  if (!v || ['TXT', 'SRV', 'CAA', 'PTR'].includes(t)) return { ok: true, message: '' }
  try {
    const d = await api<{ ok: boolean; message: string }>(
      `/api/cf/resolve?type=${encodeURIComponent(t)}&name=${encodeURIComponent(v)}&t=${Date.now()}`,
    )
    return { ok: d.ok !== false, message: d.message || '' }
  } catch {
    return { ok: true, message: '' }
  }
}
