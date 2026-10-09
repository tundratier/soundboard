import { useRef, useState } from 'react'
import { ArrowLeft, ArrowRight, ExternalLink, LayoutGrid, Play, Square, Trash2, Upload, X, Zap } from 'lucide-react'
import { t, tp } from '../i18n'
import { api } from '../shared/api'
import { SLUG_MAX, slugify } from '../shared/slug'
import { DEFAULT_TILE_COLOR, TileIconView, tileLabel } from '../shared/TileFace'
import { PLAY_MODES, type Board, type SoundInfo, type Tile } from '../shared/types'
import { SOUND_DRAG_TYPE } from './SoundsPanel'

const TILE_DRAG_TYPE = 'application/x-soundboard-tile'
const MAX_GRID = 12
const EMOJIS = '🔔 🎺 🥁 🎸 🎹 🎤 👏 😂 🤣 😱 😡 👍 👎 🔥 💥 🎉 🚨 📣 🐔 🐶 🐱 🦆 🐸 🚀 ⚡ 💀 👻 🧨 🛎️ 📯 🎬 🍺 ❤️ ⭐ ✅ ❌ 🤔 😴 🙏 🎵'.split(' ')

const newId = () => [...crypto.getRandomValues(new Uint8Array(6))].map((b) => b.toString(16).padStart(2, '0')).join('')

export function BoardEditor({
  board,
  setBoard,
  otherIds,
  canMoveLeft,
  canMoveRight,
  onMove,
  onDelete,
  sounds,
  icons,
  onIconsChanged,
  notify,
}: {
  board: Board
  setBoard: (b: Board) => void
  otherIds: string[] // IDs (URL paths) of the other boards, to keep the path unique
  canMoveLeft: boolean
  canMoveRight: boolean
  onMove: (dir: -1 | 1) => void
  onDelete?: () => void // undefined: the last remaining board cannot be deleted
  sounds: SoundInfo[]
  icons: string[]
  onIconsChanged: () => void
  notify: (msg: string, error?: boolean) => void
}) {
  const [selected, setSelected] = useState<string | null>(null)
  const [over, setOver] = useState<string | null>(null)

  const tileAt = (x: number, y: number) => board.tiles.find((t) => t.x === x && t.y === y)
  const sel = board.tiles.find((t) => t.id === selected) ?? null
  const hidden = board.tiles.filter((t) => t.x >= board.cols || t.y >= board.rows)

  const updateTile = (id: string, patch: Partial<Tile>) =>
    setBoard({ ...board, tiles: board.tiles.map((t) => (t.id === id ? { ...t, ...patch } : t)) })

  const removeTile = (id: string) => {
    setBoard({ ...board, tiles: board.tiles.filter((t) => t.id !== id) })
    setSelected(null)
  }

  const createTile = (x: number, y: number, sound = ''): Tile => {
    const t: Tile = {
      id: newId(),
      x,
      y,
      sound,
      label: sound ? sound.replace(/\.[^.]+$/, '') : undefined,
      pan: 0,
      gain: 1,
      mode: 'oneshot',
    }
    setBoard({ ...board, tiles: [...board.tiles, t] })
    return t
  }

  const onDrop = (e: React.DragEvent, x: number, y: number) => {
    e.preventDefault()
    setOver(null)
    const existing = tileAt(x, y)
    const sound = e.dataTransfer.getData(SOUND_DRAG_TYPE)
    const moving = e.dataTransfer.getData(TILE_DRAG_TYPE)
    if (sound) {
      if (existing) {
        updateTile(existing.id, { sound, label: existing.label || sound.replace(/\.[^.]+$/, '') })
        setSelected(existing.id)
      } else {
        setSelected(createTile(x, y, sound).id)
      }
    } else if (moving) {
      const t = board.tiles.find((tt) => tt.id === moving)
      if (!t || (t.x === x && t.y === y)) return
      // Swap if the target is occupied
      setBoard({
        ...board,
        tiles: board.tiles.map((tt) => {
          if (tt.id === t.id) return { ...tt, x, y }
          if (existing && tt.id === existing.id) return { ...tt, x: t.x, y: t.y }
          return tt
        }),
      })
    }
  }

  const resize = (cols: number, rows: number) => {
    const c = Math.max(1, Math.min(MAX_GRID, cols || 1))
    const r = Math.max(1, Math.min(MAX_GRID, rows || 1))
    setBoard({ ...board, cols: c, rows: r })
  }

  const cells = []
  for (let y = 0; y < board.rows; y++) {
    for (let x = 0; x < board.cols; x++) {
      const tile = tileAt(x, y)
      const key = `${x},${y}`
      cells.push(
        <div
          key={key}
          className={`cell${over === key ? ' dragover' : ''}${tile && tile.id === selected ? ' selected' : ''}`}
          draggable={!!tile}
          onDragStart={(e) => tile && (e.dataTransfer.setData(TILE_DRAG_TYPE, tile.id), (e.dataTransfer.effectAllowed = 'move'))}
          onDragOver={(e) => {
            e.preventDefault()
            setOver(key)
          }}
          onDragLeave={() => setOver((o) => (o === key ? null : o))}
          onDrop={(e) => onDrop(e, x, y)}
          onClick={() => setSelected(tile ? tile.id : createTile(x, y).id)}
          title={tile ? t('editor.cellEdit') : t('editor.cellCreate')}
        >
          {tile ? (
            <div className="tile" style={{ ['--tile-color' as string]: tile.color || DEFAULT_TILE_COLOR }}>
              <TileIconView tile={tile} />
              <span className="tile-label">{tileLabel(tile)}</span>
            </div>
          ) : (
            <span>+</span>
          )}
        </div>,
      )
    }
  }

  return (
    <section className="card">
      <div className="row">
        <h2 className="grow">
          <LayoutGrid size={18} /> {t('editor.title')}
        </h2>
        <button className="btn small" disabled={!canMoveLeft} onClick={() => onMove(-1)} title={t('editor.moveLeft')}>
          <ArrowLeft size={13} />
        </button>
        <button className="btn small" disabled={!canMoveRight} onClick={() => onMove(1)} title={t('editor.moveRight')}>
          <ArrowRight size={13} />
        </button>
        <a className="btn small" href={`/boards/${encodeURIComponent(board.id)}`} target="_blank" rel="noreferrer" title={t('editor.openTitle')}>
          <ExternalLink size={13} /> {t('editor.open')}
        </a>
        <button className="btn small danger" disabled={!onDelete} onClick={onDelete} title={onDelete ? t('editor.deleteTitle') : t('editor.lastBoard')}>
          <Trash2 size={13} /> {t('common.delete')}
        </button>
      </div>
      <div className="row">
        <div className="field" style={{ flex: 3 }}>
          <label>{t('editor.name')}</label>
          <input type="text" value={board.name} onChange={(e) => setBoard({ ...board, name: e.target.value })} />
        </div>
        <div className="field" style={{ flex: 3 }}>
          <label>{t('editor.path')}</label>
          <SlugInput value={board.id} otherIds={otherIds} onCommit={(id) => setBoard({ ...board, id })} notify={notify} />
        </div>
      </div>
      <div className="row">
        <div className="field">
          <label>{t('editor.cols')}</label>
          <input type="number" min={1} max={MAX_GRID} value={board.cols} onChange={(e) => resize(+e.target.value, board.rows)} />
        </div>
        <div className="field">
          <label>{t('editor.rows')}</label>
          <input type="number" min={1} max={MAX_GRID} value={board.rows} onChange={(e) => resize(board.cols, +e.target.value)} />
        </div>
      </div>
      {hidden.length > 0 && (
        <div className="note warn">{tp('editor.hidden', hidden.length)}</div>
      )}
      <div className="editor-grid" style={{ gridTemplateColumns: `repeat(${board.cols}, minmax(0, 1fr))` }}>
        {cells}
      </div>
      {sel && (
        <TileEditor
          key={sel.id}
          tile={sel}
          sounds={sounds}
          icons={icons}
          onIconsChanged={onIconsChanged}
          onChange={(p) => updateTile(sel.id, p)}
          onRemove={() => removeTile(sel.id)}
          onClose={() => setSelected(null)}
          notify={notify}
        />
      )}
    </section>
  )
}

function TileEditor({
  tile,
  sounds,
  icons,
  onIconsChanged,
  onChange,
  onRemove,
  onClose,
  notify,
}: {
  tile: Tile
  sounds: SoundInfo[]
  icons: string[]
  onIconsChanged: () => void
  onChange: (p: Partial<Tile>) => void
  onRemove: () => void
  onClose: () => void
  notify: (msg: string, error?: boolean) => void
}) {
  const iconRef = useRef<HTMLInputElement>(null)
  const knownSound = sounds.some((s) => s.name === tile.sound)
  const mode = PLAY_MODES.find((m) => m.value === tile.mode)
  const balLabel =
    tile.pan === 0
      ? t('editor.balanceCenter')
      : tile.pan < 0
        ? t('editor.balanceLeft', { percent: Math.round(-tile.pan * 100) })
        : t('editor.balanceRight', { percent: Math.round(tile.pan * 100) })

  const uploadIcon = async (files: File[]) => {
    if (!files.length) return
    try {
      const r = await api.upload('icons', files)
      if (r.errors.length) notify(r.errors.join('; '), true)
      if (r.saved[0]) onChange({ icon: { kind: 'file', value: r.saved[0] } })
    } catch (e) {
      notify((e as Error).message, true)
    }
    onIconsChanged()
  }

  const preview = () => api.preview(tile.sound, tile.gain, tile.pan).catch((e) => notify(e.message, true))

  return (
    <div className="card" style={{ background: 'var(--bg-3)' }}>
      <div className="row">
        <h3 className="grow">
          {t('tileEditor.title', { x: tile.x + 1, y: tile.y + 1 })}
        </h3>
        <button className="btn small" onClick={onClose} title={t('tileEditor.close')}>
          <X size={14} />
        </button>
      </div>

      <div className="field">
        <label>{t('tileEditor.sound')}</label>
        <div className="row" style={{ flexWrap: 'nowrap' }}>
          <select value={tile.sound} onChange={(e) => onChange({ sound: e.target.value, label: tile.label || e.target.value.replace(/\.[^.]+$/, '') })}>
            <option value="">{t('tileEditor.noSound')}</option>
            {!knownSound && tile.sound && <option value={tile.sound}>{t('tileEditor.soundMissing', { sound: tile.sound })}</option>}
            {sounds
              .filter((s) => !s.error)
              .map((s) => (
                <option key={s.name} value={s.name}>
                  {s.name}
                </option>
              ))}
          </select>
          <button className="btn" disabled={!knownSound} onClick={preview} title={t('tileEditor.preview')}>
            <Play size={14} />
          </button>
          <button className="btn" onClick={() => api.previewStop()} title={t('common.previewStop')}>
            <Square size={14} />
          </button>
        </div>
      </div>

      <div className="field">
        <label>{t('tileEditor.label')}</label>
        <input type="text" value={tile.label ?? ''} placeholder={t('tileEditor.labelPlaceholder')} onChange={(e) => onChange({ label: e.target.value })} />
      </div>

      <div className="field">
        <div className="label">{t('tileEditor.icon')}</div>
        <div className="emoji-row">
          <button className={!tile.icon ? 'active' : ''} onClick={() => onChange({ icon: undefined })} title={t('tileEditor.noIcon')}>
            ∅
          </button>
          {EMOJIS.map((em) => (
            <button key={em} className={tile.icon?.kind === 'emoji' && tile.icon.value === em ? 'active' : ''} onClick={() => onChange({ icon: { kind: 'emoji', value: em } })}>
              {em}
            </button>
          ))}
        </div>
        <div className="row" style={{ marginTop: 4 }}>
          <input
            type="text"
            style={{ width: 120 }}
            placeholder={t('tileEditor.customEmoji')}
            value={tile.icon?.kind === 'emoji' ? tile.icon.value : ''}
            onChange={(e) => onChange({ icon: e.target.value ? { kind: 'emoji', value: e.target.value.slice(0, 8) } : undefined })}
          />
          <button className="btn small" onClick={() => iconRef.current?.click()}>
            <Upload size={13} /> {t('tileEditor.uploadImage')}
          </button>
          <input ref={iconRef} type="file" hidden accept=".png,.jpg,.jpeg,.gif,.webp,.svg,image/*" onChange={(e) => (uploadIcon([...(e.target.files ?? [])]), (e.target.value = ''))} />
        </div>
        {icons.length > 0 && (
          <div className="icon-files" style={{ marginTop: 4 }}>
            {icons.map((name) => (
              <div key={name} className={`ic${tile.icon?.kind === 'file' && tile.icon.value === name ? ' active' : ''}`} onClick={() => onChange({ icon: { kind: 'file', value: name } })} title={name}>
                <img src={`/icons/${encodeURIComponent(name)}`} alt={name} />
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="field">
        <label>
          <span>{t('tileEditor.balance')}</span>
          <span>{balLabel}</span>
        </label>
        <input type="range" min={-1} max={1} step={0.05} value={tile.pan} onChange={(e) => onChange({ pan: +e.target.value })} onDoubleClick={() => onChange({ pan: 0 })} />
      </div>
      <div className="field">
        <label>
          <span>{t('tileEditor.volume')}</span>
          <span>{Math.round(tile.gain * 100)} %</span>
        </label>
        <input type="range" min={0} max={2} step={0.05} value={tile.gain} onChange={(e) => onChange({ gain: +e.target.value })} onDoubleClick={() => onChange({ gain: 1 })} />
      </div>

      <div className="row">
        <div className="field">
          <label>{t('tileEditor.mode')}</label>
          <select value={tile.mode} onChange={(e) => onChange({ mode: e.target.value as Tile['mode'] })}>
            {PLAY_MODES.map((m) => (
              <option key={m.value} value={m.value}>
                {t(m.label)}
              </option>
            ))}
          </select>
        </div>
        <div className="field" style={{ flex: 0 }}>
          <label>{t('tileEditor.color')}</label>
          <div className="row" style={{ flexWrap: 'nowrap' }}>
            <input type="color" value={tile.color || DEFAULT_TILE_COLOR} onChange={(e) => onChange({ color: e.target.value })} />
            {tile.color && (
              <button className="btn small" onClick={() => onChange({ color: undefined })} title={t('tileEditor.defaultColor')}>
                ∅
              </button>
            )}
          </div>
        </div>
      </div>
      <div className="note">{mode && t(mode.hint)}</div>

      <div className="row">
        <button className="btn" disabled={!knownSound} onClick={() => api.trigger(tile.id).catch((e) => notify(e.message, true))} title={t('tileEditor.liveTitle')}>
          <Zap size={14} /> {t('tileEditor.live')}
        </button>
        <div className="grow" />
        <button className="btn danger" onClick={onRemove}>
          <Trash2 size={14} /> {t('tileEditor.remove')}
        </button>
      </div>
    </div>
  )
}

/**
 * Editor for the board's URL path. Typing is free; the value is normalized and checked for duplicates
 * only when the field loses focus, so the path never changes while the user is still typing.
 */
function SlugInput({
  value,
  otherIds,
  onCommit,
  notify,
}: {
  value: string
  otherIds: string[]
  onCommit: (id: string) => void
  notify: (msg: string, error?: boolean) => void
}) {
  const [text, setText] = useState(value)
  // follow external changes (e.g. switching boards)
  const [lastValue, setLastValue] = useState(value)
  if (value !== lastValue) {
    setLastValue(value)
    setText(value)
  }

  const commit = () => {
    const id = text.trim() === '' ? value : slugify(text)
    if (otherIds.includes(id)) {
      notify(t('editor.pathTaken', { id }), true)
      setText(value)
      return
    }
    setText(id)
    if (id !== value) onCommit(id)
  }

  return (
    <div className="slug-input">
      <span>/boards/</span>
      <input
        type="text"
        value={text}
        maxLength={SLUG_MAX + 10}
        onChange={(e) => setText(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
        spellCheck={false}
      />
    </div>
  )
}
