import { useEffect, useRef } from 'react'

/** Ruft save(value) verzögert auf, wenn sich value ändert (nicht beim ersten Rendern). */
export function useDebouncedSave<T>(value: T | null, save: (v: T) => void, delayMs = 500) {
  const first = useRef(true)
  const saveRef = useRef(save)
  saveRef.current = save
  useEffect(() => {
    if (value === null) return
    if (first.current) {
      first.current = false
      return
    }
    const t = window.setTimeout(() => saveRef.current(value), delayMs)
    return () => window.clearTimeout(t)
  }, [value, delayMs])
}
