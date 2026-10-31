import type { ToolId } from './toolsMeta'
import type { Provider } from './Sidebar'
import { HomeView } from './toolsMeta'
import { ToolView } from '../tools/registry'

// Thin dispatcher: labels/icons live in toolsMeta, the screens live in src/tools/*.
export function Tools({ id, onGo }: { id: ToolId; onGo?: (p: Provider | null, t: ToolId | null) => void }) {
  if (id === 'home') return <HomeView go={onGo} />
  return <ToolView id={id} />
}
