import type { Tile } from './types'

export const DEFAULT_TILE_COLOR = '#3b4252'

/** Icon einer Kachel: Emoji oder hochgeladenes Bild. */
export function TileIconView({ tile }: { tile: Pick<Tile, 'icon' | 'label' | 'sound'> }) {
  const icon = tile.icon
  if (icon?.kind === 'file') return <img className="tile-img" src={`/icons/${encodeURIComponent(icon.value)}`} alt="" draggable={false} />
  if (icon?.kind === 'emoji' && icon.value) return <span className="tile-emoji">{icon.value}</span>
  return <span className="tile-emoji dim">♪</span>
}

export function tileLabel(t: Pick<Tile, 'label' | 'sound'>): string {
  return t.label || t.sound.replace(/\.[^.]+$/, '') || '(leer)'
}
