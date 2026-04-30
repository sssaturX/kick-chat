'use client'

import { useState } from 'react'
import Link from 'next/link'
import Logo from './Logo'
import type { Lang, Translation } from '@/app/lib/translations'

const navLinks = [
  { key: 'home' as const, href: '#hero' },
  { key: 'what' as const, href: '#what-is-saturx' },
  { key: 'features' as const, href: '#features' },
  { key: 'faq' as const, href: '#faq' },
  { key: 'getSaturx' as const, href: '#get-saturx' },
]

type HeaderCopy = {
  nav: Translation['nav']
  cta: Translation['cta']
}

export default function Header({ lang, copy }: { lang: Lang; copy: HeaderCopy }) {
  const [menuOpen, setMenuOpen] = useState(false)
  const nextLang = lang === 'ru' ? 'en' : 'ru'

  function persistLang() {
    try {
      localStorage.setItem('saturx-lang', nextLang)
    } catch {
      // ignore
    }
  }

  return (
    <header className="site-header sticky top-0 z-50 flex h-16 items-center justify-between px-4 sm:px-6 lg:px-8">
      <Link href="#hero" className="flex items-center gap-2.5 rounded-xl">
        <Logo className="w-8 h-8 text-saturx-green" />
        <span className="text-lg font-bold tracking-tight text-saturx-text sm:text-xl">SaturX</span>
      </Link>

      <nav className="hidden items-center gap-7 lg:flex">
        {navLinks.map(({ key, href }) => (
          <Link
            key={href}
            href={href}
            className="nav-link"
          >
            {copy.nav[key]}
          </Link>
        ))}
      </nav>

      <div className="flex items-center gap-3">
        <Link
          href={`/${nextLang}`}
          onClick={persistLang}
          className="premium-button rounded-lg border border-saturx-border px-3 py-2 text-sm font-semibold text-saturx-muted hover:border-saturx-dim hover:bg-saturx-elevated hover:text-saturx-text"
          title={lang === 'ru' ? 'English' : 'Русский'}
        >
          {lang === 'ru' ? 'EN' : 'RU'}
        </Link>
        <Link
          href="#get-saturx"
          className="premium-button primary-glow hidden rounded-lg bg-saturx-green px-5 py-2.5 text-sm font-bold text-[#0f0f0f] hover:bg-saturx-green-hover sm:inline-flex"
        >
          <span className="relative z-10">{copy.cta}</span>
        </Link>

        {/* Mobile menu button */}
        <button
          type="button"
          onClick={() => setMenuOpen((o) => !o)}
          className="rounded-lg p-2 text-saturx-text hover:bg-saturx-elevated lg:hidden"
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
        <div className="site-header absolute left-0 right-0 top-full flex flex-col gap-2 border-b border-saturx-border px-4 py-4 lg:hidden">
          {navLinks.map(({ key, href }) => (
            <Link
              key={href}
              href={href}
              onClick={() => setMenuOpen(false)}
              className="rounded-lg px-3 py-2.5 text-sm font-semibold text-saturx-muted hover:bg-saturx-elevated hover:text-saturx-text"
            >
              {copy.nav[key]}
            </Link>
          ))}
          <Link
            href="#get-saturx"
            onClick={() => setMenuOpen(false)}
            className="premium-button primary-glow mt-2 rounded-lg bg-saturx-green px-4 py-3 text-center text-sm font-bold text-[#0f0f0f]"
          >
            {copy.cta}
          </Link>
        </div>
      )}
    </header>
  )
}
