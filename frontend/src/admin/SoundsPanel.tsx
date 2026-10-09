import { useRef, useState } from 'react'
import { Music, Play, Square, Trash2, Upload } from 'lucide-react'
import { api } from '../shared/api'
import type { SoundInfo } from '../shared/types'

export const SOUND_DRAG_TYPE = 'application/x-soundboard-sound'

export function SoundsPanel({
  sounds,
  usedSounds,
  onChanged,
  notify,
}: {
  sounds: SoundInfo[]
  usedSounds: Set<string>
  onChanged: () => void
  notify: (msg: string, error?: boolean) => void
}) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)
  const [busy, setBusy] = useState(false)

  const upload = async (files: File[]) => {
    if (!files.length) return
    setBusy(true)
    try {
      const r = await api.upload('sounds', files)
      notify(`${r.saved.length} Sound(s) hochgeladen${r.errors.length ? ` - Fehler: ${r.errors.join('; ')}` : ''}`, r.errors.length > 0)
    } catch (e) {
      notify(`Upload fehlgeschlagen: ${(e as Error).message}`, true)
    } finally {
      setBusy(false)
      onChanged()
    }
  }

  const remove = async (s: SoundInfo) => {
    const extra = usedSounds.has(s.name) ? '\n\nDer Sound wird von einer Kachel verwendet; sie bleibt, ist dann aber stumm.' : ''
    if (!confirm(`„${s.name}“ löschen?${extra}`)) return
    try {
      await api.deleteSound(s.name)
    } catch (e) {
      notify((e as Error).message, true)
    }
    onChanged()
  }

  return (
    <section className="card">
      <h2>
        <Music size={18} /> Sounds <span className="note">({sounds.length})</span>
      </h2>
      <div
        className={`dropzone${over ? ' dragover' : ''}`}
        onClick={() => fileRef.current?.click()}
        onDragOver={(e) => {
          if (e.dataTransfer.types.includes('Files')) {
            e.preventDefault()
            setOver(true)
          }
        }}
        onDragLeave={() => setOver(false)}
        onDrop={(e) => {
          e.preventDefault()
          setOver(false)
          upload([...e.dataTransfer.files])
        }}
      >
        <Upload size={16} /> {busy ? 'Lade hoch …' : 'Dateien hierher ziehen oder klicken (wav, mp3, flac, ogg)'}
        <input
          ref={fileRef}
          type="file"
          multiple
          hidden
          accept=".wav,.mp3,.flac,.ogg,audio/*"
          onChange={(e) => {
            upload([...(e.target.files ?? [])])
            e.target.value = ''
          }}
        />
      </div>
      {sounds.length === 0 && <div className="note">Noch keine Sounds. Du kannst auch Dateien direkt in den Sounds-Ordner kopieren.</div>}
      <div className="sound-list">
        {sounds.map((s) => (
          <div
            key={s.name}
            className={`sound-item${s.error ? ' error' : ''}`}
            draggable={!s.error}
            onDragStart={(e) => {
              e.dataTransfer.setData(SOUND_DRAG_TYPE, s.name)
              e.dataTransfer.effectAllowed = 'copy'
            }}
            title={s.error ?? 'In ein Raster-Feld ziehen'}
          >
            <button className="btn small" disabled={!!s.error} onClick={() => api.preview(s.name, 1, 0).catch((e) => notify(e.message, true))} title="Vorhören (Monitor)">
              <Play size={13} />
            </button>
            <span className="name">{s.name}</span>
            {s.error ? (
              <span className="note bad">{s.error}</span>
            ) : (
              <span className="note">{(s.durationMs / 1000).toFixed(1)} s</span>
            )}
            {usedSounds.has(s.name) && <span className="dot ok" title="Wird verwendet" />}
            <button className="btn small danger" onClick={() => remove(s)} title="Löschen">
              <Trash2 size={13} />
            </button>
          </div>
        ))}
      </div>
      {sounds.length > 0 && (
        <button className="btn small" style={{ alignSelf: 'flex-start' }} onClick={() => api.previewStop()}>
          <Square size={13} /> Vorhören stoppen
        </button>
      )}
    </section>
  )
}
