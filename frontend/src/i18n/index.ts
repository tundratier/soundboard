// Frontend localization: German and English, no library.
//
// The language is an app-wide setting made in the desktop app (-lang flag or tray menu), so the page asks the
// backend which one to use (GET /api/language) before it renders. Changing it in the tray makes the backend
// announce "language" and the pages reload (see useBoardSocket and Admin). Texts live in de.ts / en.ts.

import { de } from './de'
import { en } from './en'

export type Lang = 'de' | 'en'
export type MessageKey = keyof typeof de
/** Base name of a plural message: 'boards.tiles' for the keys 'boards.tiles_one' and 'boards.tiles_other'. */
export type PluralKey = { [K in MessageKey]: K extends `${infer B}_other` ? B : never }[MessageKey]

type Params = Record<string, string | number>

const catalogs: Record<Lang, Record<string, string>> = { de, en }
let lang: Lang = 'en'

export const getLanguage = (): Lang => lang

function apply(l: Lang) {
  lang = l
  document.documentElement.lang = l
}

/** Loads the language from the backend; without a backend answer the browser language decides. Call before rendering. */
export async function initLanguage(): Promise<void> {
  try {
    const res = await fetch('/api/language')
    if (res.ok) {
      const l = (await res.json()).language
      if (l === 'de' || l === 'en') return apply(l)
    }
  } catch {
    /* backend not reachable: fall through */
  }
  apply(navigator.language?.toLowerCase().startsWith('de') ? 'de' : 'en')
}

function format(text: string, params?: Params): string {
  if (!params) return text
  return text.replace(/\{(\w+)\}/g, (m, name: string) => (name in params ? String(params[name]) : m))
}

/** Text for a key in the current language; {placeholders} are filled from params. */
export function t(key: MessageKey, params?: Params): string {
  return format(catalogs[lang][key] ?? catalogs.en[key] ?? key, params)
}

/** Plural-aware text: picks <key>_one or <key>_other for count; {count} is filled automatically. */
export function tp(key: PluralKey, count: number, params?: Params): string {
  const form = new Intl.PluralRules(lang).select(count)
  const cat = catalogs[lang]
  const text = cat[`${key}_${form}`] ?? cat[`${key}_other`] ?? catalogs.en[`${key}_other`] ?? key
  return format(text, { count, ...params })
}
