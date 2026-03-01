'use client'

import { useState } from 'react'
import Link from 'next/link'
import Logo from './Logo'
import { useLang } from '@/app/context/LangContext'

const navLinks = [
  { key: 'home' as const, href: '#hero' },
  { key: 'features' as const, href: '#features' },
  { key: 'screenshots' as const, href: '#screenshots' },
  { key: 'getSaturx' as const, href: '#get-saturx' },
]

export default function Header() {
  const { t, lang, setLang } = useLang()
  const [menuOpen, setMenuOpen] = useState(false)

  return (
    <header className="sticky top-0 z-50 h-16 bg-white border-b border-saturx-border flex items-center justify-between px-4 sm:px-6 lg:px-8">
      <Link href="#hero" className="flex items-center gap-2.5">
        <Logo className="w-8 h-8 text-saturx-purple" />
        <span className="text-xl font-semibold tracking-tight text-saturx-dark">SaturX</span>
      </Link>

      <nav className="hidden md:flex items-center gap-8">
        {navLinks.map(({ key, href }) => (
          <Link
            key={href}
            href={href}
            className="text-sm text-saturx-muted hover:text-saturx-dark transition-colors"
          >
            {t.nav[key]}
          </Link>
        ))}
      </nav>

      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => setLang(lang === 'ru' ? 'en' : 'ru')}
          className="text-sm text-saturx-muted hover:text-saturx-dark px-2.5 py-1.5 rounded-md border border-saturx-border hover:border-saturx-muted transition-colors"
          title={lang === 'ru' ? 'English' : 'Русский'}
        >
          {lang === 'ru' ? 'EN' : 'RU'}
        </button>
        <Link
          href="#get-saturx"
          className="hidden sm:inline-flex px-5 py-2.5 rounded-lg bg-saturx-purple hover:bg-saturx-purple-light text-white text-sm font-medium transition-colors"
        >
          {t.cta}
        </Link>

        {/* Mobile menu button */}
        <button
          type="button"
          onClick={() => setMenuOpen((o) => !o)}
          className="md:hidden p-2 rounded-lg text-saturx-dark hover:bg-saturx-bg-subtle"
          aria-expanded={menuOpen}
          aria-label="Menu"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            {menuOpen ? (
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            ) : (
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            )}
          </svg>
        </button>
      </div>

      {/* Mobile menu */}
      {menuOpen && (
        <div className="absolute top-full left-0 right-0 md:hidden bg-white border-b border-saturx-border py-4 px-4 flex flex-col gap-2">
          {navLinks.map(({ key, href }) => (
            <Link
              key={href}
              href={href}
              onClick={() => setMenuOpen(false)}
              className="text-sm text-saturx-muted hover:text-saturx-dark py-2"
            >
              {t.nav[key]}
            </Link>
          ))}
          <Link
            href="#get-saturx"
            onClick={() => setMenuOpen(false)}
            className="mt-2 px-4 py-2.5 rounded-lg bg-saturx-purple text-white text-sm font-medium text-center"
          >
            {t.cta}
          </Link>
        </div>
      )}
    </header>
  )
}
