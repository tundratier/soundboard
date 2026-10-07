import { useCallback, useEffect, useRef, useState } from 'react'
import type { ServerEvent } from './types'

/**
 * Hält eine WebSocket-Verbindung zum Backend (mit Auto-Reconnect) und liefert die gerade spielenden Kacheln.
 * onBoardChanged wird bei "board"-Ereignissen und nach jedem (Wieder-)Verbinden aufgerufen.
 */
export function useBoardSocket(onBoardChanged: () => void) {
  const [connected, setConnected] = useState(false)
  const [playing, setPlaying] = useState<Set<string>>(new Set())
  const wsRef = useRef<WebSocket | null>(null)
  const cbRef = useRef(onBoardChanged)
  cbRef.current = onBoardChanged

  useEffect(() => {
    let closed = false
    let retry = 0
    let timer: number | undefined

    const connect = () => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const ws = new WebSocket(`${proto}://${location.host}/ws`)
      wsRef.current = ws
      ws.onopen = () => {
        retry = 0
        setConnected(true)
        cbRef.current()
      }
      ws.onmessage = (ev) => {
        let m: ServerEvent
        try {
          m = JSON.parse(ev.data)
        } catch {
          return
        }
        switch (m.type) {
          case 'hello':
            setPlaying(new Set(m.playing))
            break
          case 'started':
            setPlaying((p) => new Set(p).add(m.tile))
            break
          case 'stopped':
            setPlaying((p) => {
              const n = new Set(p)
              n.delete(m.tile)
              return n
            })
            break
          case 'board':
            cbRef.current()
            break
        }
      }
      ws.onclose = () => {
        setConnected(false)
        wsRef.current = null
        if (!closed) {
          retry = Math.min(retry + 1, 6)
          timer = window.setTimeout(connect, 500 * retry)
        }
      }
      ws.onerror = () => ws.close()
    }
    connect()
    return () => {
      closed = true
      window.clearTimeout(timer)
      wsRef.current?.close()
    }
  }, [])

  const send = useCallback((msg: object): boolean => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg))
      return true
    }
    return false
  }, [])

  return { connected, playing, send }
}
