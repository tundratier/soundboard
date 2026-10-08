import { useCallback, useEffect, useRef, useState } from 'react'
import NoSleep from 'nosleep.js'

const STORAGE_KEY = 'soundboard.keepAwake'

function readPreference(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) !== '0' // on by default
  } catch {
    return true
  }
}

/**
 * Keeps the screen on while the board is open.
 *
 * The Screen Wake Lock API only exists in secure contexts (HTTPS or localhost); the board is normally served
 * over plain http://<LAN-IP>, where it is unavailable. nosleep.js therefore falls back to a tiny hidden looping
 * video, which keeps phones awake over plain HTTP too. Both variants need a user gesture, so the first touch
 * anywhere on the page activates it (and re-activates it after the tab was in the background).
 */
export function useKeepAwake() {
  const noSleep = useRef<NoSleep | null>(null)
  const [wanted, setWanted] = useState(readPreference)
  const [active, setActive] = useState(false)
  const wantedRef = useRef(wanted)
  wantedRef.current = wanted

  const enable = useCallback(async () => {
    if (!noSleep.current || noSleep.current.isEnabled) return
    try {
      await noSleep.current.enable()
      setActive(true)
    } catch {
      setActive(false) // no user gesture yet or unsupported; retried on the next touch
    }
  }, [])

  useEffect(() => {
    const ns = new NoSleep()
    noSleep.current = ns

    const onGesture = () => {
      if (wantedRef.current) void enable()
    }
    const onVisible = () => {
      if (document.visibilityState === 'visible') {
        setActive(ns.isEnabled)
        if (wantedRef.current) void enable()
      }
    }
    document.addEventListener('pointerdown', onGesture, { capture: true })
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      document.removeEventListener('pointerdown', onGesture, { capture: true })
      document.removeEventListener('visibilitychange', onVisible)
      ns.disable()
      noSleep.current = null
    }
  }, [enable])

  const toggle = useCallback(() => {
    const next = !wantedRef.current
    setWanted(next)
    try {
      localStorage.setItem(STORAGE_KEY, next ? '1' : '0')
    } catch {
      /* private mode: preference just is not persisted */
    }
    if (next) {
      void enable()
    } else {
      noSleep.current?.disable()
      setActive(false)
    }
  }, [enable])

  return { wanted, active, toggle }
}
