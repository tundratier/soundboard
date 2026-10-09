import { Sun, SunDim } from 'lucide-react'
import { t } from '../i18n'
import { useKeepAwake } from './useKeepAwake'

/** Sun button that keeps the screen on (see useKeepAwake); shared by the board and the overview. */
export function KeepAwakeButton() {
  const { wanted, active, toggle } = useKeepAwake()
  const title = !wanted ? t('keepAwake.off') : active ? t('keepAwake.on') : t('keepAwake.pending')
  return (
    <button className={`btn icon${active ? ' active' : ''}`} onClick={toggle} title={title}>
      {wanted ? <Sun size={18} /> : <SunDim size={18} />}
    </button>
  )
}
