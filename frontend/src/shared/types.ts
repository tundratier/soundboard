// Spiegel der Go-Strukturen (backend/internal/store, audio, library, server).
// Bei Änderungen am Backend-Datenmodell hier nachziehen.

export type PlayMode = 'oneshot' | 'restart' | 'toggle' | 'loop'

export const PLAY_MODES: { value: PlayMode; label: string; hint: string }[] = [
  { value: 'oneshot', label: 'Einmal', hint: 'Jeder Druck startet den Sound (auch überlappend)' },
  { value: 'restart', label: 'Neustart', hint: 'Erneuter Druck startet von vorn' },
  { value: 'toggle', label: 'Start/Stopp', hint: 'Zweiter Druck stoppt den Sound' },
  { value: 'loop', label: 'Schleife', hint: 'Läuft in Endlosschleife, zweiter Druck stoppt' },
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
  board: Board
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
  error?: string
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
}

export interface BoardResponse {
  board: Board
  playing: string[]
  missing: string[]
}

export type ServerEvent =
  | { type: 'hello'; playing: string[] }
  | { type: 'started'; tile: string }
  | { type: 'stopped'; tile: string }
  | { type: 'board' }
