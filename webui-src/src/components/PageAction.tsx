import { createContext, useContext, useEffect, useMemo, useRef } from 'react'
import type { ReactNode } from 'react'
import type { Icon } from '@phosphor-icons/react'

// The topbar owns the primary actions of a page; tools register them here so they
// sit next to the title instead of a full-width bar over the list. On phones the
// same list becomes a floating "+" button with a bottom sheet.
export type PageAction = { label: string; icon?: Icon; onClick: () => void; disabled?: boolean }

type Ctx = { set: (a: PageAction[]) => void }
const PageActionCtx = createContext<Ctx>({ set: () => {} })
const EMPTY: PageAction[] = []

export function usePageActions(list: PageAction[]) {
  const { set } = useContext(PageActionCtx)
  const ref = useRef(list)
  ref.current = list
  const key = list.map((a) => a.label + (a.disabled ? '!' : '')).join('|')
  useEffect(() => {
    if (!key) {
      set(EMPTY)
      return
    }
    const register = () => set(ref.current.map((a) => ({ ...a, onClick: () => ref.current.find((x) => x.label === a.label)?.onClick() })))
    register()
    return () => set(EMPTY)
  }, [set, key])
}

export function usePageAction(a: PageAction | null) {
  usePageActions(a ? [a] : EMPTY)
}

export function PageActionProvider({ children, set }: { children: ReactNode; set: (a: PageAction[]) => void }) {
  const v = useMemo(() => ({ set }), [set])
  return <PageActionCtx.Provider value={v}>{children}</PageActionCtx.Provider>
}
