import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

export type Mode = 'light' | 'dark'
const KEY = 'cg_mode'

function apply(mode: Mode) {
  document.documentElement.dataset.mode = mode
  try {
    localStorage.setItem(KEY, mode)
    const tg = (window as any).Telegram?.WebApp
    // setHeaderColor/setBackgroundColor need Bot API 6.1+; older clients warn.
    const ok = tg?.isVersionAtLeast ? tg.isVersionAtLeast('6.1') : false
    if (ok) {
      tg?.setHeaderColor?.(mode === 'dark' ? '#0e1116' : '#ffffff')
      tg?.setBackgroundColor?.(mode === 'dark' ? '#0e1116' : '#ffffff')
    }
  } catch {
    /* ignore */
  }
}

export function initMode(): Mode {
  let mode: Mode = 'light'
  try {
    const saved = localStorage.getItem(KEY)
    if (saved === 'dark' || saved === 'light') mode = saved
  } catch {
    /* ignore */
  }
  apply(mode)
  return mode
}

const Ctx = createContext<{ mode: Mode; setMode: (m: Mode) => void; toggle: () => void }>({
  mode: 'light',
  setMode: () => {},
  toggle: () => {},
})

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<Mode>(() => (document.documentElement.dataset.mode === 'dark' ? 'dark' : 'light'))
  useEffect(() => apply(mode), [mode])
  const value = useMemo(
    () => ({ mode, setMode, toggle: () => setMode((m) => (m === 'dark' ? 'light' : 'dark')) }),
    [mode],
  )
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useTheme() {
  return useContext(Ctx)
}
