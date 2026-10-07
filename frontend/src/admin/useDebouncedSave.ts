import { useEffect, useRef } from 'react'

/** Calls save(value) with a delay when value changes (not on the first render). */
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
