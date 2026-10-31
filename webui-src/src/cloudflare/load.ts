import { api } from '../lib'
import type { CFAcc, CFRec, CFZone, EmailData, FavItem, LbEntry, LbGroup, ZoneInfo } from './model'
import type { View } from './model'

export type Load =
  | { kind: 'zones'; zones: CFZone[]; accs: CFAcc[] }
  | { kind: 'accounts'; accs: CFAcc[] }
  | { kind: 'favs'; favs: FavItem[] }
  | {
      kind: 'zone'
      zone: ZoneInfo
      records: CFRec[]
      settings?: { groups: any[]; dev: boolean; paused: boolean }
      email?: EmailData
      traffic?: { days: any; total: any; top: any[] }
    }
  | { kind: 'lb'; zone: ZoneInfo; groups: LbGroup[] }
  | { kind: 'lbgroup'; zone: ZoneInfo; name: string; entries: LbEntry[] }

// One loader per view so Cloudflare.tsx only applies the result.
export async function loadView(
  view: View,
  tab: string,
  days: number | string,
  signal: AbortSignal,
): Promise<Load> {
  if (view.v === 'zones' || view.v === 'lbzones') {
    const d = await api<{ zones: CFZone[]; accounts: CFAcc[] }>('/api/cf/zones', undefined, signal)
    return { kind: 'zones', zones: d.zones || [], accs: d.accounts || [] }
  }
  if (view.v === 'accounts') {
    const d = await api<{ accounts: CFAcc[] }>('/api/cf/accounts', undefined, signal)
    return { kind: 'accounts', accs: d.accounts || [] }
  }
  if (view.v === 'favs') {
    const d = await api<{ items: FavItem[] }>('/api/cf/favs', undefined, signal)
    return { kind: 'favs', favs: d.items || [] }
  }
  if (view.v === 'zone') {
    const q = 'acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid)
    const d = await api<{ zone: ZoneInfo; records: CFRec[] }>('/api/cf/records?' + q, undefined, signal)
    const out: Load = { kind: 'zone', zone: d.zone, records: d.records || [] }
    if (tab === 'settings') {
      const s = await api<any>('/api/cf/settings?' + q, undefined, signal)
      out.settings = { groups: s.groups || [], dev: !!s.dev, paused: !!s.paused }
    } else if (tab === 'email') {
      out.email = await api<EmailData>('/api/cf/email?' + q, undefined, signal)
    } else if (tab === 'traffic') {
      const t = await api<any>('/api/cf/traffic?' + q + '&days=' + days, undefined, signal)
      out.traffic = { days: t.days, total: t.total, top: t.top || [] }
    }
    return out
  }
  if (view.v === 'lb') {
    const d = await api<{ zone: ZoneInfo; groups: LbGroup[] }>(
      '/api/cf/lbgroups?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid),
      undefined,
      signal,
    )
    return { kind: 'lb', zone: d.zone, groups: d.groups || [] }
  }
  if (view.v === 'lbgroup') {
    const d = await api<{ zone: ZoneInfo; name: string; entries: LbEntry[] }>(
      '/api/cf/lb?acc=' + view.acc + '&zid=' + encodeURIComponent(view.zid) + '&name=' + encodeURIComponent(view.name),
      undefined,
      signal,
    )
    return { kind: 'lbgroup', zone: d.zone, name: d.name, entries: d.entries || [] }
  }
  throw new Error('view ناشناخته')
}
