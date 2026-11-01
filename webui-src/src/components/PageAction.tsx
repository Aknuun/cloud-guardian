import { createContext, useContext, useEffect, useMemo, useRef } from 'react'
import type { ReactNode } from 'react'
import type { Icon } from '@phosphor-icons/react'

// The topbar owns the single primary action of a page; tools register it here so it
// sits next to the title instead of a full-width bar over the list.
export type PageAction = { label: string; icon?: Icon; onClick: () => void; disabled?: boolean }

type Ctx = { set: (a: PageAction | null) => void }
const PageActionCtx = createContext<Ctx>({ set: () => {} })

export function usePageAction(a: PageAction | null) {
  const { set } = useContext(PageActionCtx)
  const ref = useRef(a)
  ref.current = a
  const label = a?.label
  const disabled = a?.disabled
  useEffect(() => {
    if (!label) { set(null); return }
    set({ label, icon: ref.current?.icon, disabled, onClick: () => ref.current?.onClick() })
    return () => set(null)
  }, [set, label, disabled])
}

export function PageActionProvider({ children, set }: { children: ReactNode; set: (a: PageAction | null) => void }) {
  const v = useMemo(() => ({ set }), [set])
  return <PageActionCtx.Provider value={v}>{children}</PageActionCtx.Provider>
}
