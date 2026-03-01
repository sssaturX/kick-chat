'use client'

import { createContext, useContext, useState, useCallback, useEffect } from 'react'
import { Lang, translations } from '@/app/lib/translations'

export type T = (typeof translations)[Lang]

const LangContext = createContext<{
  lang: Lang
  setLang: (lang: Lang) => void
  t: T
} | null>(null)

const STORAGE_KEY = 'saturx-lang'

export function LangProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = useState<Lang>('en')

  useEffect(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY) as Lang | null
      if (stored === 'ru' || stored === 'en') setLangState(stored)
    } catch {
      // ignore
    }
  }, [])

  const setLang = useCallback((next: Lang) => {
    setLangState(next)
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // ignore
    }
  }, [])

  const t = translations[lang]

  return (
    <LangContext.Provider value={{ lang, setLang, t }}>
      {children}
    </LangContext.Provider>
  )
}

export function useLang() {
  const ctx = useContext(LangContext)
  if (!ctx) throw new Error('useLang must be used within LangProvider')
  return ctx
}
