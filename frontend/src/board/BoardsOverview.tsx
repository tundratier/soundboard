import { useCallback, useEffect, useState } from 'react'
import { api } from '../shared/api'
import { KeepAwakeButton } from '../shared/KeepAwakeButton'
import { DEFAULT_TILE_COLOR } from '../shared/TileFace'
import type { BoardSummary } from '../shared/types'
import { useBoardSocket } from '../shared/useBoardSocket'

/** /boards: list of all boards; each card links to /boards/<id>. */
export function BoardsOverview() {
  const [boards, setBoards] = useState<BoardSummary[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const refetch = useCallback(() => {
    api
      .boards()
      .then((b) => {
        setBoards(b)
        setError(null)
      })
      .catch((e) => setError(String(e.message ?? e)))
  }, [])

  const { connected } = useBoardSocket(refetch)

  useEffect(() => {
    document.title = 'Boards'
  }, [])

  if (!boards) {
    return <div className="center-msg">{error ? `Keine Verbindung: ${error}` : 'Lade …'}</div>
  }

  return (
    <div className="board-page">
      <header className="board-head">
        <h1>Boards</h1>
        <span className={`dot ${connected ? 'ok' : 'bad'}`} title={connected ? 'Verbunden' : 'Getrennt - verbinde neu …'} />
        <div className="grow" />
        <KeepAwakeButton />
      </header>
      {!connected && <div className="banner">Verbindung getrennt - versuche erneut …</div>}
      <main className="board-cards">
        {boards.map((b) => (
          <a key={b.id} className="board-card" href={`/boards/${encodeURIComponent(b.id)}`}>
            <BoardPreview board={b} />
            <span className="board-card-name">{b.name}</span>
            <span className="board-card-meta">
              {b.tiles.length} {b.tiles.length === 1 ? 'Kachel' : 'Kacheln'} · {b.cols}×{b.rows}
            </span>
          </a>
        ))}
      </main>
    </div>
  )
}

/** Miniature of a board's grid: one small cell per tile, in the tile's color. */
function BoardPreview({ board }: { board: BoardSummary }) {
  const byPos = new Map(board.tiles.map((t) => [`${t.x},${t.y}`, t]))
  const cells = []
  for (let y = 0; y < board.rows; y++) {
    for (let x = 0; x < board.cols; x++) {
      const t = byPos.get(`${x},${y}`)
      cells.push(
        <i
          key={`${x},${y}`}
          className={t ? 'on' : undefined}
          style={t ? { ['--tile-color' as string]: t.color || DEFAULT_TILE_COLOR } : undefined}
        >
          {t?.icon?.kind === 'emoji' ? t.icon.value : null}
        </i>,
      )
    }
  }
  return (
    <span
      className="board-preview"
      style={{ gridTemplateColumns: `repeat(${board.cols}, minmax(0, 1fr))`, aspectRatio: `${board.cols} / ${board.rows}` }}
    >
      {cells}
    </span>
  )
}
