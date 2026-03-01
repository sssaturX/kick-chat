'use client'

import Link from 'next/link'
import Logo from './Logo'
import { useLang } from '@/app/context/LangContext'

export default function Footer() {
  const { t } = useLang()

  return (
    <footer className="py-12 px-4 sm:px-6 lg:px-8 border-t border-saturx-border bg-saturx-bg-subtle">
      <div className="max-w-6xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-6">
        <Link href="#hero" className="flex items-center gap-2 text-saturx-dark">
          <Logo className="w-7 h-7 text-saturx-purple" />
          <span className="font-semibold">SaturX</span>
        </Link>
        <div className="flex gap-8 text-sm text-saturx-muted">
          <Link href="#features" className="hover:text-saturx-dark transition-colors">{t.footer.features}</Link>
          <Link href="#screenshots" className="hover:text-saturx-dark transition-colors">{t.footer.screenshots}</Link>
          <Link href="#get-saturx" className="hover:text-saturx-dark transition-colors">{t.footer.getSaturx}</Link>
        </div>
      </div>
      <p className="max-w-6xl mx-auto mt-6 text-center text-xs text-saturx-muted">
        {t.footer.tagline}
      </p>
    </footer>
  )
}
