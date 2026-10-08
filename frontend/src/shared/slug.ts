// Mirror of store.Slugify / store.UniqueSlug in the Go backend (board IDs double as URL paths).

export const SLUG_MAX = 40

export function slugify(name: string): string {
  const repl: Record<string, string> = { ä: 'ae', ö: 'oe', ü: 'ue', ß: 'ss', Ä: 'ae', Ö: 'oe', Ü: 'ue' }
  const s = name
    .replace(/[äöüßÄÖÜ]/g, (c) => repl[c])
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, SLUG_MAX)
    .replace(/-+$/g, '')
  return s || 'board'
}

/** Returns base, or base-2, base-3 … until it is not in `taken`. */
export function uniqueSlug(base: string, taken: Set<string>): string {
  if (!taken.has(base)) return base
  for (let i = 2; ; i++) {
    const suffix = `-${i}`
    const cand = base.slice(0, SLUG_MAX - suffix.length).replace(/-+$/g, '') + suffix
    if (!taken.has(cand)) return cand
  }
}
