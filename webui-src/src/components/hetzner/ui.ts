

export const LOC_FA: Record<string, string> = {
  nbg1: 'نورنبرگ، آلمان',
  fsn1: 'فالکنشتاین، آلمان',
  hel1: 'هلسینکی، فنلاند',
  ash: 'اشبرن، آمریکا',
  hil: 'هیلزبورو، آمریکا',
  sin: 'سنگاپور',
  nyc1: 'نیویورک، آمریکا',
}

export function locFa(name: string, city?: string, country?: string) {
  if (LOC_FA[name]) return LOC_FA[name]
  if (city && country) return city + '، ' + country
  return city || country || name
}

export const STATUS: Record<string, string> = {
  running: 'روشن',
  off: 'خاموش',
  starting: 'در حال روشن شدن',
  stopping: 'در حال خاموش شدن',
  rebuilding: 'ریبیلد',
  migrating: 'انتقال',
  deleting: 'در حال حذف',
  unknown: 'نامشخص',
}

export function badgeVariant(st: string): any {
  if (st === 'running') return 'success'
  if (st === 'off') return 'neutral'
  if (st === 'starting' || st === 'migrating' || st === 'rebuilding') return 'warning'
  if (st === 'unknown' || st === 'deleting') return 'error'
  return 'secondary'
}
