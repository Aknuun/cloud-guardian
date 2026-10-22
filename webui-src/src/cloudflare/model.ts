import { faNum } from '../lib'

export type CFAcc = { i: number; name: string; zones?: number }
export type CFZone = { id: string; name: string; status: string; acc: number; accName: string }
export type CFRec = {
  id: string
  type: string
  name: string
  content: string
  ttl: number
  proxied: boolean
  priority?: number | null
  label: string
  fav: boolean
}
export type ZoneInfo = {
  id: string
  name: string
  status: string
  paused: boolean
  plan: string
  created: string
  account: string
  ns: string[]
}
export type SetItem = { key: string; label: string; value: string; kind: 'bool' | 'enum'; values?: { v: string; l: string }[] }
export type SetGroup = { key: string; title: string; items: SetItem[] }
export type EmailRule = { id: string; label: string; dest: string; enabled?: boolean }
export type EmailData = {
  available: boolean
  enabled: boolean
  status?: string
  rules: EmailRule[]
  catchAll: string
  dests: { email: string; verified: boolean }[]
}
export type FavItem = {
  acc: number
  zid: string
  rid: string
  zone: string
  label: string
  type?: string
  content?: string
  name?: string
  missing: boolean
}
export type LbGroup = { name: string; on: number; off: number; total: number }
export type LbEntry = {
  on: boolean
  id: string
  type: string
  content: string
  proxied: boolean
  ttl: number
  weight: number
  spare?: { type: string; content: string }
}
export type View =
  | { v: 'zones' }
  | { v: 'accounts' }
  | { v: 'favs' }
  | { v: 'search' }
  | { v: 'zone'; acc: number; zid: string }
  | { v: 'lbzones' }
  | { v: 'lb'; acc: number; zid: string }
  | { v: 'lbgroup'; acc: number; zid: string; name: string }
export type Field = {
  key: string
  label: string
  placeholder?: string
  value?: string
  password?: boolean
  select?: { value: string; label: string }[]
  hint?: string
  validate?: (v: string) => string | undefined
}
export type PickItem = { id: string; title: string; sub?: string }
export type Dlg =
  | { k: 'confirm'; title: string; desc?: string; danger?: boolean; okText?: string; onOk: () => void | Promise<void> }
  | { k: 'form'; title: string; desc?: string; fields: Field[]; okText?: string; onSubmit: (v: Record<string, string>) => void | Promise<void> }
  | { k: 'pick'; title: string; items: PickItem[]; onPick: (id: string) => void }
  | { k: 'list'; title: string; rows: { title: string; sub?: string }[]; empty?: string }
  | { k: 'record'; acc: number; zid: string; zone: string; rec: CFRec }
  | { k: 'editor'; acc: number; zid: string; zone: string; rec?: CFRec }
  | { k: 'spare'; acc: number; zid: string; name: string; entry: LbEntry }

export const ZONE_BADGE: Record<string, 'success' | 'warning' | 'secondary' | 'neutral' | 'error'> = {
  active: 'success',
  paused: 'warning',
  pending: 'warning',
  initializing: 'secondary',
  moved: 'neutral',
  deleted: 'error',
}
export const TTL_STEPS = [1, 120, 300, 1800, 3600, 21600, 86400]
export const TYPES = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS', 'SRV']
export const PROXYABLE = ['A', 'AAAA', 'CNAME']
export const TAB_KEYS = ['records', 'settings', 'email', 'traffic'] as const
export type Tab = (typeof TAB_KEYS)[number]

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const HOST_RE = /^(?=.{1,253}$)(?!-)[a-z0-9_-]{1,63}(?:\.(?!-)[a-z0-9_-]{1,63})*\.?$/i
const NAME_RE = /^(@|\*|(?=.{1,253}$)(?!-)[a-z0-9_*-]{1,63}(?:\.(?!-)[a-z0-9_*-]{1,63})*)$/i

export function statusLabel(s: string) {
  switch (s) {
    case 'active':
      return 'فعال'
    case 'paused':
      return 'متوقف'
    case 'pending':
      return 'در انتظار'
    case 'initializing':
      return 'راه‌اندازی'
    case 'moved':
      return 'منتقل‌شده'
    case 'deleted':
      return 'حذف‌شده'
  }
  return s
}
export function ttlLabel(t: number) {
  if (!t || t === 1) return 'خودکار'
  if (t < 3600) return faNum(t / 60) + ' دقیقه'
  return faNum(t / 3600) + ' ساعت'
}
export function relName(name: string, zone: string) {
  if (!zone) return name
  if (name === zone) return '@'
  if (name.endsWith('.' + zone)) return name.slice(0, -(zone.length + 1))
  return name
}
export function isIPv4(v: string) {
  const p = v.split('.')
  return p.length === 4 && p.every((x) => /^\d{1,3}$/.test(x) && Number(x) <= 255)
}
export function isIPv6(v: string) {
  return v.includes(':') && /^[0-9a-f:]+$/i.test(v)
}
export function isHost(v: string) {
  return HOST_RE.test(v.trim())
}
export function validateRecord(type: string, name: string, content: string, priority: string, isNew: boolean) {
  if (isNew && !name.trim()) return 'نام رکورد را وارد کن.'
  if (isNew && !NAME_RE.test(name.trim())) return 'نام معتبر نیست؛ از حروف، عدد، خط تیره و نقطه استفاده کن.'
  if (!content.trim()) return 'مقدار رکورد را وارد کن.'
  if (type === 'A' && !isIPv4(content.trim())) return 'آی‌پی IPv4 معتبر نیست (مثلاً 1.2.3.4).'
  if (type === 'AAAA' && !isIPv6(content.trim())) return 'آی‌پی IPv6 معتبر نیست.'
  if (['CNAME', 'MX', 'NS'].includes(type) && !HOST_RE.test(content.trim()))
    return 'میزبان معتبر نیست (مثلاً host.example.com).'
  if (type === 'TXT' && content.length > 2048) return 'متن TXT حداکثر ۲۰۴۸ کاراکتر است.'
  if (type === 'MX' && priority !== '') {
    const n = Number(priority)
    if (!Number.isInteger(n) || n < 0 || n > 65535) return 'اولویت باید عددی بین ۰ تا ۶۵۵۳۵ باشد.'
  }
  return ''
}
export function riskySetting(key: string, value: string, current: string) {
  if (key === 'ssl' && ['off', 'flexible'].includes(value))
    return 'با این حالت، ارتباط بین کلادفلر و سرور شما رمزنگاری نمی‌شود و ممکن است سایت از دسترس خارج شود.'
  if (key === 'min_tls_version' && ['1.0', '1.1'].includes(value))
    return 'نسخه‌های قدیمی TLS امنیت سایت را کاهش می‌دهند.'
  if (key === 'security_level' && ['off', 'low'].includes(value))
    return 'با کاهش سطح امنیت، حملات خودکار کمتر مسدود می‌شوند.'
  if (key === 'always_use_https' && value === 'off')
    return 'با خاموش‌شدن این گزینه، بازدید بدون HTTPS ممکن می‌شود.'
  if (key === 'automatic_https_rewrites' && value === 'off')
    return 'لینک‌های HTTP به‌صورت خودکار به HTTPS تبدیل نمی‌شوند.'
  if (key === 'tls_1_3' && value === 'off' && current === 'on')
    return 'TLS 1.3 امن‌ترین نسخه است؛ خاموش‌کردن آن توصیه نمی‌شود.'
  return ''
}
export function isEmail(v: string) {
  return EMAIL_RE.test(v.trim())
}

export function isView(v: any): v is View {
  if (!v || typeof v !== 'object') return false
  if (!['zones', 'accounts', 'favs', 'search', 'zone', 'lbzones', 'lb', 'lbgroup'].includes(v.v)) return false
  if (['zone', 'lb', 'lbgroup'].includes(v.v) && (typeof v.acc !== 'number' || typeof v.zid !== 'string')) return false
  if (v.v === 'lbgroup' && typeof v.name !== 'string') return false
  return true
}

export function restoreCfState(): { view: View; stack: View[]; tab: Tab } | null {
  try {
    const raw = sessionStorage.getItem('cg_cf_state')
    if (!raw) return null
    const d = JSON.parse(raw)
    if (!isView(d.view)) return null
    const stack = Array.isArray(d.stack) ? d.stack.filter(isView) : []
    const tab: Tab = TAB_KEYS.includes(d.tab) ? d.tab : 'records'
    return { view: d.view, stack, tab }
  } catch {
    return null
  }
}

export function splitFavorites<T extends { fav: boolean }>(list: T[]) {
  return {
    favs: list.filter((x) => x.fav),
    rest: list.filter((x) => !x.fav),
  }
}
