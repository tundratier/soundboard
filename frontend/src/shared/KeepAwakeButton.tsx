import { Sun, SunDim } from 'lucide-react'
import { useKeepAwake } from './useKeepAwake'

/** Sun button that keeps the screen on (see useKeepAwake); shared by the board and the overview. */
export function KeepAwakeButton() {
  const { wanted, active, toggle } = useKeepAwake()
  const title = !wanted
    ? 'Display darf ausgehen – antippen, um es wach zu halten'
    : active
      ? 'Display bleibt an – antippen zum Ausschalten'
      : 'Display wird beim nächsten Antippen wach gehalten'
  return (
    <button className={`btn icon${active ? ' active' : ''}`} onClick={toggle} title={title}>
      {wanted ? <Sun size={18} /> : <SunDim size={18} />}
    </button>
  )
}
