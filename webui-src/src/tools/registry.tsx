import { lazy, Suspense } from 'react'
import type { ComponentType, LazyExoticComponent } from 'react'
import { Loader } from '../components/ui'
import type { ToolId } from '../components/toolsMeta'

// Every tool is its own lazy chunk: opening one section never downloads the rest.
const VIEWS: Partial<Record<ToolId, LazyExoticComponent<ComponentType>>> = {
  srv: lazy(() => import('./srv')),
  rem: lazy(() => import('./rem')),
  pg: lazy(() => import('./pg')),
  um: lazy(() => import('./um')),
  dom: lazy(() => import('./dom')),
  hf: lazy(() => import('./hf')),
  hfd: lazy(() => import('./hfd')),
  hook: lazy(() => import('./hook')),
  mail: lazy(() => import('./mail')),
  set: lazy(() => import('./set')),
  fo: lazy(() => import('./fo')),
  hub: lazy(() => import('./hub')),
  hs: lazy(() => import('../components/HostStudio').then((m) => ({ default: m.HostStudio }))),
  cfcheck: lazy(() => import('../components/CfCheck').then((m) => ({ default: m.CfCheckView }))),
}

export function ToolView({ id }: { id: ToolId }) {
  const C = VIEWS[id]
  if (!C) return null
  return (
    <Suspense fallback={<div className="cc-center"><Loader size="lg" /></div>}>
      <C />
    </Suspense>
  )
}
