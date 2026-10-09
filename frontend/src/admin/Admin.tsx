import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ExternalLink, Globe, Headphones, Monitor, Mic, Plus, Square, TriangleAlert } from 'lucide-react'
import { getLanguage, t, tp } from '../i18n'
import { api } from '../shared/api'
import { slugify, uniqueSlug } from '../shared/slug'
import type { AdminState, AudioSettings, Board, DeviceInfo, EngineStatus, RoleStatus, ServerSettings } from '../shared/types'
import { BoardEditor } from './BoardEditor'
import { Meter } from './Meter'
import { SoundsPanel } from './SoundsPanel'
import { useDebouncedSave } from './useDebouncedSave'

type Notify = (msg: string, error?: boolean) => void

export function Admin() {
  const [state, setState] = useState<AdminState | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [status, setStatus] = useState<EngineStatus | null>(null)
  const [toast, setToast] = useState<{ msg: string; error: boolean } | null>(null)

  // Local drafts: taken from the server state only on the first load, then saved automatically.
  const [audio, setAudio] = useState<AudioSettings | null>(null)
  const [boards, setBoards] = useState<Board[] | null>(null)
  const [active, setActive] = useState(0) // index of the selected board tab
  const [server, setServer] = useState<ServerSettings | null>(null)

  const toastTimer = useRef<number>(undefined)
  const notify: Notify = useCallback((msg, error = false) => {
    setToast({ msg, error })
    window.clearTimeout(toastTimer.current)
    toastTimer.current = window.setTimeout(() => setToast(null), error ? 7000 : 3500)
  }, [])

  const load = useCallback(async () => {
    try {
      const s = await api.state()
      if (s.language !== getLanguage()) {
        location.reload() // the language was changed in the tray menu; drafts are auto-saved, so nothing is lost
        return
      }
      setState(s)
      setLoadError(null)
      setAudio((a) => a ?? s.config.audio)
      setBoards((b) => b ?? s.config.boards)
      setServer((v) => v ?? s.config.server)
    } catch (e) {
      setLoadError((e as Error).message)
    }
  }, [])

  useEffect(() => {
    load()
    const timer = window.setInterval(load, 2500) // keep device list, sounds and cable detection up to date
    return () => window.clearInterval(timer)
  }, [load])

  useEffect(() => {
    let stop = false
    let timer: number
    const tick = async () => {
      if (!document.hidden) {
        try {
          const s = await api.status()
          if (!stop) setStatus(s)
        } catch {
          /* ignore */
        }
      }
      if (!stop) timer = window.setTimeout(tick, 120)
    }
    tick()
    return () => {
      stop = true
      window.clearTimeout(timer)
    }
  }, [])

  useEffect(() => {
    document.title = t('admin.documentTitle')
  }, [])

  useDebouncedSave(audio, (a) => api.putAudio(a).catch((e) => notify(t('admin.audioSaveFailed', { error: e.message }), true)), 400)
  useDebouncedSave(boards, (b) => api.putBoards(b).catch((e) => notify(t('admin.boardsSaveFailed', { error: e.message }), true)), 500)

  const usedSounds = useMemo(() => new Set((boards ?? []).flatMap((b) => b.tiles.map((tile) => tile.sound))), [boards])

  if (!state || !audio || !boards || !server) {
    return <div className="center-msg">{loadError ? t('admin.backendUnreachable', { error: loadError }) : t('common.loading')}</div>
  }
  const live = status ?? state.status
  const current = boards[Math.min(active, boards.length - 1)]
  const currentIdx = boards.indexOf(current)

  const setCurrent = (b: Board) => setBoards(boards.map((x, i) => (i === currentIdx ? b : x)))
  const addBoard = () => {
    const name = t('admin.newBoardName', { n: boards.length + 1 })
    const id = uniqueSlug(slugify(name), new Set(boards.map((b) => b.id)))
    setBoards([...boards, { id, name, cols: 4, rows: 3, tiles: [] }])
    setActive(boards.length)
  }
  const deleteBoard = () => {
    if (boards.length < 2) return
    if (!confirm(t('admin.deleteBoardConfirm', { name: current.name, tiles: tp('boards.tiles', current.tiles.length) }))) return
    setBoards(boards.filter((_, i) => i !== currentIdx))
    setActive(Math.max(0, currentIdx - 1))
  }
  const moveBoard = (dir: -1 | 1) => {
    const to = currentIdx + dir
    if (to < 0 || to >= boards.length) return
    const next = [...boards]
    ;[next[currentIdx], next[to]] = [next[to], next[currentIdx]]
    setBoards(next)
    setActive(to)
  }

  return (
    <div className="admin">
      <header className="admin-head">
        <h1>{t('admin.heading')}</h1>
        <span className="note">{/^\d/.test(state.version) ? `v${state.version}` : state.version}</span>
        <div className="grow" />
        <a className="btn" href="/boards" target="_blank" rel="noreferrer">
          <ExternalLink size={14} /> {t('admin.openBoards')}
        </a>
        <button className="btn danger" onClick={() => api.stopAll()}>
          <Square size={14} /> {t('common.stopAll')}
        </button>
      </header>

      <div className="admin-cols">
        <div className="stack">
          <AudioPanel audio={audio} setAudio={setAudio} devices={state.devices} status={live} cableOutId={state.cable.outputId} cableInId={state.cable.inputId} devicesError={state.devicesError} />
          <WindowsPanel state={state} notify={notify} />
          <NetworkPanel state={state} server={server} setServer={setServer} notify={notify} />
        </div>
        <div className="stack">
          <div>
            <div className="tabs" role="tablist">
              {boards.map((b, i) => (
                <button key={b.id} role="tab" aria-selected={i === currentIdx} className={`tab${i === currentIdx ? ' active' : ''}`} onClick={() => setActive(i)} title={b.name}>
                  {b.name}
                </button>
              ))}
              <button className="btn small" onClick={addBoard} title={t('admin.newBoard')}>
                <Plus size={14} /> {t('admin.boardButton')}
              </button>
            </div>
            <BoardEditor
              key={currentIdx}
              board={current}
              setBoard={setCurrent}
              otherIds={boards.filter((b) => b !== current).map((b) => b.id)}
              canMoveLeft={currentIdx > 0}
              canMoveRight={currentIdx < boards.length - 1}
              onMove={moveBoard}
              onDelete={boards.length > 1 ? deleteBoard : undefined}
              sounds={state.sounds}
              icons={state.icons}
              onIconsChanged={load}
              notify={notify}
            />
          </div>
          <SoundsPanel sounds={state.sounds} usedSounds={usedSounds} onChanged={load} notify={notify} />
          <div className="note">
            {t('admin.dataFolder')} <code>{state.dataDir}</code>
          </div>
        </div>
      </div>
      {toast && <div className={`toast${toast.error ? ' error' : ''}`}>{toast.msg}</div>}
    </div>
  )
}

// ---------- Audio ----------

/** UI text for the backend's status error codes; unknown codes fall back to the backend's (English) message. */
function roleErrorText(role: RoleStatus): string {
  switch (role.errorCode) {
    case 'device_missing':
      return t('audio.errDeviceMissing')
    case 'feedback':
      return t('audio.errFeedback')
    case 'device_stopped':
      return t('audio.errStopped')
    case 'open_failed':
      return t('audio.errOpenFailed', { error: role.error ?? '' })
    default:
      return role.error || t('audio.starting')
  }
}

function RoleLine({ role, configured }: { role: RoleStatus; configured: boolean }) {
  if (!configured) return <div className="status-line"><span className="dot" /> <span className="dim">{t('audio.notSelected')}</span></div>
  if (role.running) return <div className="status-line"><span className="dot ok" /> {t('audio.running')}</div>
  return (
    <div className="status-line">
      <span className="dot bad" /> <span className="note bad">{roleErrorText(role)}</span>
    </div>
  )
}

function deviceLabel(d: DeviceInfo, extra = '') {
  return `${d.name}${d.isDefault ? t('audio.defaultSuffix') : ''}${extra}`
}

function AudioPanel({
  audio,
  setAudio,
  devices,
  status,
  cableInId,
  cableOutId,
  devicesError,
}: {
  audio: AudioSettings
  setAudio: (a: AudioSettings) => void
  devices: DeviceInfo[]
  status: EngineStatus
  cableInId?: string
  cableOutId?: string
  devicesError?: string
}) {
  const capture = devices.filter((d) => d.kind === 'capture')
  const playback = devices.filter((d) => d.kind === 'playback')
  const set = (p: Partial<AudioSettings>) => setAudio({ ...audio, ...p })
  const gain = (key: 'micGain' | 'fxGainMain' | 'fxGainMonitor', label: string) => (
    <div className="field">
      <label>
        <span>{label}</span>
        <span>{Math.round(audio[key] * 100)} %</span>
      </label>
      <input type="range" min={0} max={2} step={0.05} value={audio[key]} onChange={(e) => set({ [key]: +e.target.value })} onDoubleClick={() => set({ [key]: 1 })} />
    </div>
  )
  const mainIsCable = audio.mainDeviceId !== '' && audio.mainDeviceId === cableInId
  const missing = (id: string, list: DeviceInfo[]) => id !== '' && !list.some((d) => d.id === id)

  return (
    <section className="card">
      <h2>
        <Headphones size={18} /> {t('audio.title')}
      </h2>
      {devicesError && <div className="note bad">{t('audio.deviceList', { error: devicesError })}</div>}

      <div className="field">
        <label>
          <span>
            <Mic size={13} /> {t('audio.input')}
          </span>
        </label>
        <select value={audio.inputDeviceId} onChange={(e) => set({ inputDeviceId: e.target.value })}>
          <option value="">{t('audio.noMic')}</option>
          {missing(audio.inputDeviceId, capture) && <option value={audio.inputDeviceId}>{t('audio.deviceUnavailable')}</option>}
          {capture.map((d) => (
            <option key={d.id} value={d.id} disabled={d.id === cableOutId}>
              {deviceLabel(d, d.id === cableOutId ? t('audio.cableOutNote') : '')}
            </option>
          ))}
        </select>
        <RoleLine role={status.input} configured={audio.inputDeviceId !== ''} />
        <Meter value={status.micPeak} label={t('audio.micLevel')} />
      </div>

      <div className="field">
        <label>
          <span>
            <Headphones size={13} /> {t('audio.main')}
          </span>
        </label>
        <select value={audio.mainDeviceId} onChange={(e) => set({ mainDeviceId: e.target.value })}>
          <option value="">{t('audio.noOutput')}</option>
          {missing(audio.mainDeviceId, playback) && <option value={audio.mainDeviceId}>{t('audio.deviceUnavailable')}</option>}
          {playback.map((d) => (
            <option key={d.id} value={d.id}>
              {deviceLabel(d, d.id === cableInId ? t('audio.cableRecommended') : '')}
            </option>
          ))}
        </select>
        <RoleLine role={status.main} configured={audio.mainDeviceId !== ''} />
        <Meter value={status.mainPeak} label={t('audio.mainLevel')} />
        {audio.mainDeviceId !== '' && audio.inputDeviceId !== '' && !mainIsCable && (
          <div className="note warn">
            <TriangleAlert size={12} /> {t('audio.mainNotCable')}
          </div>
        )}
      </div>

      <div className="field">
        <label>
          <span>
            <Monitor size={13} /> {t('audio.monitor')}
          </span>
        </label>
        <select value={audio.monitorDeviceId} onChange={(e) => set({ monitorDeviceId: e.target.value })}>
          <option value="">{t('audio.noMonitor')}</option>
          {missing(audio.monitorDeviceId, playback) && <option value={audio.monitorDeviceId}>{t('audio.deviceUnavailable')}</option>}
          {playback.map((d) => (
            <option key={d.id} value={d.id}>
              {deviceLabel(d)}
            </option>
          ))}
        </select>
        <RoleLine role={status.monitor} configured={audio.monitorDeviceId !== ''} />
        <Meter value={status.monitorPeak} label={t('audio.monitorLevel')} />
        <div className="note">{t('audio.monitorNote')}</div>
        {audio.monitorDeviceId !== '' && audio.monitorDeviceId === audio.mainDeviceId && (
          <div className="note warn">
            <TriangleAlert size={12} /> {t('audio.sameDevice')}
          </div>
        )}
      </div>

      <div className="row">
        {gain('micGain', t('audio.gainMic'))}
        {gain('fxGainMain', t('audio.gainMain'))}
        {gain('fxGainMonitor', t('audio.gainMonitor'))}
      </div>

      <details>
        <summary className="note" style={{ cursor: 'pointer' }}>
          {t('audio.advanced')}
        </summary>
        <div className="row" style={{ marginTop: 8 }}>
          <div className="field">
            <label>{t('audio.buffer')}</label>
            <select value={audio.periodMs} onChange={(e) => set({ periodMs: +e.target.value })}>
              <option value={5}>{t('audio.buffer5')}</option>
              <option value={10}>{t('audio.buffer10')}</option>
              <option value={20}>{t('audio.buffer20')}</option>
              <option value={30}>{t('audio.buffer30')}</option>
            </select>
          </div>
        </div>
        <div className="note" style={{ marginTop: 6 }}>
          {t('audio.micStats', { under: status.micUnderruns, over: status.micOverruns, ppm: ((status.micRatio - 1) * 1e6).toFixed(0) })}
        </div>
      </details>
    </section>
  )
}

// ---------- Windows integration ----------

function WindowsPanel({ state, notify }: { state: AdminState; notify: Notify }) {
  const capture = state.devices.filter((d) => d.kind === 'capture')
  const [target, setTarget] = useState('')
  const [all, setAll] = useState(false)
  const effective = target || state.cable.outputId || ''

  const apply = async () => {
    try {
      await api.setDefaultDevice(effective, all ? 'all' : 'communications')
      notify(all ? t('win.setAll') : t('win.setCommunications'))
    } catch (e) {
      notify(t('win.setFailed', { error: (e as Error).message }), true)
    }
  }

  return (
    <section className="card">
      <h2>
        <Mic size={18} /> {t('win.title')}
      </h2>
      {state.cable.found ? (
        <div className="status-line">
          <span className="dot ok" /> {t('win.cableFound')}
        </div>
      ) : (
        <div className="note warn">
          <TriangleAlert size={12} /> {t('win.cableMissingBefore')}
          <a href="https://vb-audio.com/Cable/" target="_blank" rel="noreferrer">
            VB-Audio Virtual Cable
          </a>
          {t('win.cableMissingAfter')}
        </div>
      )}
      <div className="field">
        <label>{t('win.recordingDevice')}</label>
        <select value={effective} onChange={(e) => setTarget(e.target.value)}>
          <option value="">-</option>
          {capture.map((d) => (
            <option key={d.id} value={d.id}>
              {deviceLabel(d, d.id === state.cable.outputId ? t('win.appOutputSuffix') : '')}
            </option>
          ))}
        </select>
      </div>
      <label className="row note" style={{ cursor: 'pointer' }}>
        <input type="checkbox" checked={all} onChange={(e) => setAll(e.target.checked)} /> {t('win.alsoDefault')}
      </label>
      <button className="btn primary" disabled={!effective} onClick={apply} style={{ alignSelf: 'flex-start' }}>
        {t('win.setButton')}
      </button>
      <div className="note">{t('win.duckingHint')}</div>
    </section>
  )
}

// ---------- Network ----------

function NetworkPanel({
  state,
  server,
  setServer,
  notify,
}: {
  state: AdminState
  server: ServerSettings
  setServer: (s: ServerSettings) => void
  notify: Notify
}) {
  const [qrIdx, setQrIdx] = useState(0)
  const urls = state.urls
  const qrUrl = urls[Math.min(qrIdx, urls.length - 1)]
  const changed = server.port !== state.config.server.port || server.bindLan !== state.config.server.bindLan

  const save = async () => {
    try {
      await api.putServer(server)
      notify(t('net.saved'))
    } catch (e) {
      notify((e as Error).message, true)
    }
  }

  return (
    <section className="card">
      <h2>
        <Globe size={18} /> {t('net.title')}
      </h2>
      {server.bindLan && urls.length > 0 ? (
        <div className="url-list">
          <img src={api.qrUrl(qrUrl)} alt={t('net.qrAlt')} />
          <div>
            <div className="note">{t('net.openOnPhone')}</div>
            {urls.map((u, i) => (
              <code key={u} onClick={() => setQrIdx(i)} style={{ cursor: 'pointer', fontWeight: i === qrIdx ? 700 : 400 }}>
                {u}
              </code>
            ))}
            <div className="note" style={{ marginTop: 6 }}>
              {t('net.warning')}
            </div>
          </div>
        </div>
      ) : (
        <div className="note">{t('net.localOnly', { port: server.port })}</div>
      )}
      <div className="row">
        <div className="field" style={{ flex: 0, minWidth: 100 }}>
          <label>{t('net.port')}</label>
          <input type="number" min={1} max={65535} value={server.port} onChange={(e) => setServer({ ...server, port: +e.target.value })} />
        </div>
        <label className="row note" style={{ cursor: 'pointer', flex: 1 }}>
          <input type="checkbox" checked={server.bindLan} onChange={(e) => setServer({ ...server, bindLan: e.target.checked })} /> {t('net.lan')}
        </label>
        <button className="btn" disabled={!changed} onClick={save}>
          {t('net.save')}
        </button>
      </div>
      {changed && <div className="note warn">{t('net.restartNeeded')}</div>}
    </section>
  )
}
