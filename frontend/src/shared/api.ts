import type { AdminState, AudioSettings, Board, BoardResponse, BoardSummary, EngineStatus, ServerSettings } from './types'

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
  }
}

// Mutating admin requests need the header (protection against CSRF from foreign web pages).
const ADMIN_HEADERS = { 'X-Soundboard': '1' }

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(url, { ...init, headers: { ...ADMIN_HEADERS, ...init.headers } })
  if (!res.ok) {
    let msg = res.statusText
    try {
      msg = (await res.json()).error ?? msg
    } catch {
      /* not a JSON response */
    }
    throw new ApiError(msg, res.status)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})

export const api = {
  // public
  boards: () => request<BoardSummary[]>('/api/boards'),
  board: (id: string) => request<BoardResponse>(`/api/boards/${encodeURIComponent(id)}`),
  trigger: (tile: string) => request<void>(`/api/trigger/${encodeURIComponent(tile)}`, { method: 'POST' }),
  stopAll: () => request<void>('/api/stopall', { method: 'POST' }),

  // Admin
  state: () => request<AdminState>('/api/admin/state'),
  status: () => request<EngineStatus>('/api/admin/status'),
  putAudio: (a: AudioSettings) => request<AudioSettings>('/api/admin/audio', json('PUT', a)),
  putBoards: (b: Board[]) => request<Board[]>('/api/admin/boards', json('PUT', b)),
  putServer: (s: ServerSettings) =>
    request<{ server: ServerSettings; restartRequired: boolean }>('/api/admin/server', json('PUT', s)),
  preview: (sound: string, gain: number, pan: number) =>
    request<void>('/api/admin/preview', json('POST', { sound, gain, pan })),
  previewStop: () => request<void>('/api/admin/preview/stop', { method: 'POST' }),
  deleteSound: (name: string) => request<void>(`/api/admin/sounds/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  deleteIcon: (name: string) => request<void>(`/api/admin/icons/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  setDefaultDevice: (deviceId: string, role: 'communications' | 'all') =>
    request<void>('/api/admin/default-device', json('POST', { deviceId, role })),
  upload: (kind: 'sounds' | 'icons', files: File[]) => {
    const fd = new FormData()
    files.forEach((f) => fd.append('file', f))
    return request<{ saved: string[]; errors: string[] }>(`/api/admin/${kind}`, { method: 'POST', body: fd })
  },
  qrUrl: (text: string) => `/api/admin/qr?text=${encodeURIComponent(text)}`,
}
