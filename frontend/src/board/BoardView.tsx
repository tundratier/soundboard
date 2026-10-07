import { useCallback, useEffect, useState } from 'react'
import { Maximize, Minimize, Repeat, Square, TriangleAlert } from 'lucide-react'
import { api } from '../shared/api'
import { DEFAULT_TILE_COLOR, TileIconView, tileLabel } from '../shared/TileFace'
import type { BoardResponse, Tile } from '../shared/types'
import { useBoardSocket } from '../shared/useBoardSocket'

export function BoardView() {
  const [data, setData] = useState<BoardResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [fullscreen, setFullscreen] = useState(!!document.fullscreenElement)

  const refetch = useCallback(() => {
    api
      .board()
      .then((d) => {
        setData(d)
        setError(null)
      })
      .catch((e) => setError(String(e.message ?? e)))
  }, [])

  const { connected, playing, send } = useBoardSocket(refetch)

  useEffect(() => {
    const onFs = () => setFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFs)
    return () => document.removeEventListener('fullscreenchange', onFs)
  }, [])

  useEffect(() => {
    if (data) document.title = data.board.name
  }, [data])

  const trigger = (t: Tile) => {
    navigator.vibrate?.(8)
    if (!send({ type: 'trigger', tile: t.id })) api.trigger(t.id).catch(() => {})
  }
  const stopAll = () => {
    if (!send({ type: 'stopAll' })) api.stopAll().catch(() => {})
  }
  const toggleFullscreen = () => {
    if (document.fullscreenElement) document.exitFullscreen()
    else document.documentElement.requestFullscreen?.().catch(() => {})
  }

  if (!data) {
    return <div className="center-msg">{error ? `Keine Verbindung: ${error}` : 'Lade …'}</div>
  }

  const { board, missing } = data
  const missingSet = new Set(missing)
  const byPos = new Map(board.tiles.map((t) => [`${t.x},${t.y}`, t]))
  const cells = []
  for (let y = 0; y < board.rows; y++) {
    for (let x = 0; x < board.cols; x++) {
      const t = byPos.get(`${x},${y}`)
      cells.push(
        t ? (
          <TileButton key={t.id} tile={t} playing={playing.has(t.id)} missing={missingSet.has(t.id)} onPress={trigger} />
        ) : (
          <div key={`e${x},${y}`} className="tile empty" />
        ),
      )
    }
  }

  return (
    <div className="board-page">
      <header className="board-head">
        <h1>{board.name}</h1>
        <span className={`dot ${connected ? 'ok' : 'bad'}`} title={connected ? 'Verbunden' : 'Getrennt – verbinde neu …'} />
        <div className="grow" />
        <button className="btn danger" onPointerDown={stopAll} title="Alle Sounds stoppen">
          <Square size={16} /> Stopp
        </button>
        {document.fullscreenEnabled && (
          <button className="btn icon" onClick={toggleFullscreen} title="Vollbild">
            {fullscreen ? <Minimize size={18} /> : <Maximize size={18} />}
          </button>
        )}
      </header>
      {!connected && <div className="banner">Verbindung getrennt – versuche erneut …</div>}
      <main
        className="board-grid"
        style={{ gridTemplateColumns: `repeat(${board.cols}, minmax(0, 1fr))`, gridTemplateRows: `repeat(${board.rows}, minmax(0, 1fr))` }}
      >
        {cells}
      </main>
    </div>
  )
}

function TileButton({
  tile,
  playing,
  missing,
  onPress,
}: {
  tile: Tile
  playing: boolean
  missing: boolean
  onPress: (t: Tile) => void
}) {
  return (
    <button
      className={`tile${playing ? ' playing' : ''}${missing ? ' missing' : ''}`}
      style={{ ['--tile-color' as string]: tile.color || DEFAULT_TILE_COLOR }}
      // pointerdown instead of click: ~100 ms less latency on touch screens
      onPointerDown={(e) => {
        if (e.button === 0 || e.pointerType === 'touch') {
          e.preventDefault()
          if (!missing) onPress(tile)
        }
      }}
      onContextMenu={(e) => e.preventDefault()}
      disabled={missing}
      title={missing ? `Sound fehlt: ${tile.sound}` : undefined}
    >
      <TileIconView tile={tile} />
      <span className="tile-label">{tileLabel(tile)}</span>
      {missing && <TriangleAlert className="tile-badge warn" size={16} />}
      {!missing && (tile.mode === 'loop' || tile.mode === 'toggle') && <Repeat className="tile-badge" size={14} />}
    </button>
  )
}
