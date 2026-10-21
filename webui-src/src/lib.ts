export const TG: any = (window as any).Telegram && (window as any).Telegram.WebApp ? (window as any).Telegram.WebApp : null

export let token = sessionStorage.getItem('cg_token') || ''

export function setToken(v: string) {
  token = v
  if (v) sessionStorage.setItem('cg_token', v)
  else sessionStorage.removeItem('cg_token')
}

export class ApiError extends Error {}

export async function api<T = any>(path: string, body?: any): Promise<T> {
  const headers: Record<string, string> = { Authorization: 'Bearer ' + token }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const res = await fetch(path, {
    method: body === undefined ? 'GET' : 'POST',
    headers,
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
export function fallbackCopy(v: string) {
  const ta = document.createElement('textarea')
  ta.value = v
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
  } catch {
    /* ignore */
  }
  ta.remove()
}
