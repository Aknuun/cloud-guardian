export type Account = { i: number; name: string; count?: number }
export type SrvType = { id: string; name: string; cores: number; memory: number; disk: number; architecture: string }
export type Traffic = { in_gb: number; out_gb: number; total_gb: number; included_gb: number; used_pct: number; extra_gb: number }
export type Snapshot = { id: string; name: string; size_gb: number; created: string; description: string }
export type Server = {
  accName?: string
  accIdx?: number
  id: string
  name: string
  status: string
  ipv4: string
  ipv6: string
  locked: boolean
  created: string
  type: SrvType
  location: { name: string; city: string; country: string }
  image: string
  traffic: Traffic
  snapshots?: Snapshot[]
  prices?: { monthly?: string; hourly?: string }
}
export type CleanIP = { id: number; ip: string; loc: string; assignee?: string; auto?: boolean; check?: { clean: boolean; listed: string[]; unknown: boolean } }
export type OptLocation = { name: string; city: string; country: string; description: string }
export type OptType = SrvType & {
  monthly?: string
  monthly_net?: number
  category?: string
  cpu_type?: string
  storage_type?: string
  traffic?: number
}
export type OptImage = { id: string; name: string; architecture: string; os_flavor?: string }
export type Options = { locations: OptLocation[]; types: OptType[]; images: OptImage[] }
