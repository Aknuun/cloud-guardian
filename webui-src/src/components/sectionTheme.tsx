import { useMemo, useSyncExternalStore, type CSSProperties } from 'react'
import { useTheme } from './theme'

export type Provider = 'hz' | 'cf' | 'ln' | 'arv'

export type Palette = {
  id: string
  label: string
  light: [string, string]
  dark: [string, string]
  primary: [string, string]
}

export const PALETTES: Palette[] = [
  { id: 'red', label: 'قرمز', light: ['#dc2626', '#ef4444'], dark: ['#ff5c5c', '#ff8a8a'], primary: ['#b91c1c', '#dc2626'] },
  { id: 'coral', label: 'مرجانی', light: ['#e14b43', '#f59e0b'], dark: ['#ff7b72', '#fbbf24'], primary: ['#b9382e', '#b45309'] },
  { id: 'orange', label: 'نارنجی', light: ['#ea580c', '#f97316'], dark: ['#ff8a3d', '#ffb066'], primary: ['#c2410c', '#ea580c'] },
  { id: 'amber', label: 'کهربایی', light: ['#d97706', '#f59e0b'], dark: ['#fbbf24', '#fcd34d'], primary: ['#b45309', '#d97706'] },
  { id: 'gold', label: 'طلایی', light: ['#a16207', '#ca8a04'], dark: ['#eab308', '#fde047'], primary: ['#854d0e', '#a16207'] },
  { id: 'lime', label: 'لیمویی', light: ['#65a30d', '#84cc16'], dark: ['#a3e635', '#bef264'], primary: ['#4d7c0f', '#65a30d'] },
  { id: 'green', label: 'سبز', light: ['#16a34a', '#22c55e'], dark: ['#4ade80', '#86efac'], primary: ['#15803d', '#16a34a'] },
  { id: 'emerald', label: 'زمردی', light: ['#059669', '#10b981'], dark: ['#34d399', '#6ee7b7'], primary: ['#047857', '#059669'] },
  { id: 'teal', label: 'فیروزه‌ای', light: ['#0d9488', '#14b8a6'], dark: ['#2dd4bf', '#5eead4'], primary: ['#0f766e', '#0d9488'] },
  { id: 'cyan', label: 'آبی روشن', light: ['#0891b2', '#06b6d4'], dark: ['#22d3ee', '#67e8f9'], primary: ['#0e7490', '#0891b2'] },
  { id: 'sky', label: 'آسمانی', light: ['#0284c7', '#0ea5e9'], dark: ['#38bdf8', '#7dd3fc'], primary: ['#0369a1', '#0284c7'] },
  { id: 'blue', label: 'آبی', light: ['#2563eb', '#3b82f6'], dark: ['#60a5fa', '#93c5fd'], primary: ['#1d4ed8', '#2563eb'] },
  { id: 'indigo', label: 'نیلی', light: ['#4f46e5', '#6366f1'], dark: ['#818cf8', '#a5b4fc'], primary: ['#4338ca', '#4f46e5'] },
  { id: 'violet', label: 'بنفش', light: ['#7c3aed', '#8b5cf6'], dark: ['#a78bfa', '#c4b5fd'], primary: ['#6d28d9', '#7c3aed'] },
  { id: 'purple', label: 'ارغوانی', light: ['#9333ea', '#a855f7'], dark: ['#c084fc', '#d8b4fe'], primary: ['#7e22ce', '#9333ea'] },
  { id: 'fuchsia', label: 'سرخابی', light: ['#c026d3', '#d946ef'], dark: ['#e879f9', '#f0abfc'], primary: ['#a21caf', '#c026d3'] },
  { id: 'pink', label: 'صورتی', light: ['#db2777', '#ec4899'], dark: ['#f472b6', '#f9a8d4'], primary: ['#be185d', '#db2777'] },
  { id: 'rose', label: 'گلبهی', light: ['#e11d48', '#f43f5e'], dark: ['#fb7185', '#fda4af'], primary: ['#be123c', '#e11d48'] },
  { id: 'slate', label: 'خاکستری', light: ['#475569', '#64748b'], dark: ['#94a3b8', '#cbd5e1'], primary: ['#334155', '#475569'] },
  { id: 'stone', label: 'سنگی', light: ['#57534e', '#78716c'], dark: ['#a8a29e', '#d6d3d1'], primary: ['#44403c', '#57534e'] },
]

export const DEFAULT_THEME: Record<Provider, string> = {
  hz: 'coral',
  cf: 'red',
  ln: 'emerald',
  arv: 'teal',
}

const KEY = 'cg_section_theme'
const PROVIDERS: Provider[] = ['hz', 'cf', 'ln', 'arv']
type Selection = Partial<Record<Provider, string>>

function read(): Selection {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Record<string, unknown>
    if (!parsed || typeof parsed !== 'object') return {}
    const out: Selection = {}
    for (const provider of PROVIDERS) {
      const id = parsed[provider]
      if (typeof id === 'string' && PALETTES.some((p) => p.id === id)) out[provider] = id
    }
    return out
  } catch {
    return {}
  }
}

let snapshot = read()
const listeners = new Set<() => void>()

function emit() {
  for (const listener of listeners) listener()
}

function save() {
  try {
    localStorage.setItem(KEY, JSON.stringify(snapshot))
  } catch {
    /* ignore */
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function getSnapshot() {
  return snapshot
}

function setSectionTheme(provider: Provider, id: string) {
  snapshot = { ...snapshot, [provider]: id }
  save()
  emit()
}

function resetSectionTheme(provider: Provider) {
  const next = { ...snapshot }
  delete next[provider]
  snapshot = next
  save()
  emit()
}

export function useSectionTheme(provider: Provider) {
  const { mode } = useTheme()
  const selection = useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
  // panel look: one accent for every section
  const palette = PALETTES.find((p) => p.id === 'blue') || PALETTES[0]
  void selection

  const style = useMemo(
    () =>
      ({
        '--pv-accent': mode === 'dark' ? palette.dark[0] : palette.light[0],
        '--pv-accent-2': mode === 'dark' ? palette.dark[1] : palette.light[1],
        '--ds-primary': palette.primary[0],
        '--ds-primary-2': palette.primary[1],
      }) as CSSProperties,
    [palette, mode],
  )

  return {
    palette,
    style,
    select: (id: string) => setSectionTheme(provider, id),
    reset: () => resetSectionTheme(provider),
  }
}
