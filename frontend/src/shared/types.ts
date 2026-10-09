// Mirror of the Go structures (backend/internal/store, audio, library, server).
// When the backend data model changes, update this file accordingly.

import type { MessageKey } from '../i18n'

export type PlayMode = 'oneshot' | 'restart' | 'toggle' | 'loop'

/** The play modes with the keys of their (localized) name and hint; see i18n/de.ts. */
export const PLAY_MODES: { value: PlayMode; label: MessageKey; hint: MessageKey }[] = [
  { value: 'oneshot', label: 'mode.oneshot', hint: 'mode.oneshot.hint' },
  { value: 'restart', label: 'mode.restart', hint: 'mode.restart.hint' },
  { value: 'toggle', label: 'mode.toggle', hint: 'mode.toggle.hint' },
  { value: 'loop', label: 'mode.loop', hint: 'mode.loop.hint' },
]

export interface TileIcon {
  kind: 'emoji' | 'file'
  value: string
}

export interface Tile {
  id: string
  x: number
  y: number
  label?: string
  sound: string
  icon?: TileIcon
  pan: number // Balance -1 … +1
  gain: number // 0 … 2
  mode: PlayMode
  color?: string
}

export interface Board {
  id: string // URL path segment: the board is served at /boards/<id>
  name: string
  cols: number
  rows: number
  tiles: Tile[]
}

export interface AudioSettings {
  inputDeviceId: string
  mainDeviceId: string
  monitorDeviceId: string
  micGain: number
  fxGainMain: number
  fxGainMonitor: number
  periodMs: number
}

export interface ServerSettings {
  port: number
  bindLan: boolean
}

export interface Config {
  version: number
  audio: AudioSettings
  server: ServerSettings
  boards: Board[]
}

export interface DeviceInfo {
  id: string
  name: string
  kind: 'capture' | 'playback'
  isDefault: boolean
}

export interface RoleStatus {
  deviceId: string
  running: boolean
  error?: string // English message from the backend (also logged)
  errorCode?: 'device_missing' | 'feedback' | 'open_failed' | 'device_stopped' // translated for the UI
}

export interface EngineStatus {
  input: RoleStatus
  main: RoleStatus
  monitor: RoleStatus
  mainPeak: number
  micPeak: number
  monitorPeak: number
  micUnderruns: number
  micOverruns: number
  micRatio: number
}

export interface SoundInfo {
  name: string
  durationMs: number
  sizeBytes: number
  error?: string
}

export interface CableInfo {
  found: boolean
  inputId?: string
  outputId?: string
}

export interface AdminState {
  config: Config
  devices: DeviceInfo[]
  status: EngineStatus
  sounds: SoundInfo[]
  icons: string[]
  cable: CableInfo
  urls: string[]
  dataDir: string
  version: string
  playing: string[]
  devicesError?: string
  language: 'de' | 'en' // active UI language; the admin page reloads when it differs from the one it rendered in
}

export interface BoardResponse {
  board: Board
  playing: string[]
  missing: string[] // tile IDs whose sound is missing or broken
}

/** One entry of the board overview: just enough to draw a small preview of the grid. */
export interface BoardSummary {
  id: string
  name: string
  cols: number
  rows: number
  tiles: { x: number; y: number; color?: string; icon?: TileIcon }[]
}

export type ServerEvent =
  | { type: 'hello'; playing: string[] }
  | { type: 'started'; tile: string }
  | { type: 'stopped'; tile: string }
  | { type: 'board' }
  | { type: 'language' } // the UI language was changed in the tray menu: reload to render in the new one
