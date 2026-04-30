import Link from 'next/link'
import Logo from './Logo'
import type { Translation } from '@/app/lib/translations'

export default function Footer({ t }: { t: Translation }) {
  return (
    <footer className="footer-shell reveal border-t border-saturx-border px-4 py-14 sm:px-6 lg:px-8">
      <div className="section-container grid gap-10 md:grid-cols-[1.1fr_1fr_1fr]">
        <div>
          <Link href="#hero" className="flex items-center gap-2 rounded-xl text-saturx-text">
            <Logo className="w-7 h-7 text-saturx-green" />
            <span className="text-base font-bold tracking-tight">SATURX</span>
          </Link>
          <p className="mt-4 max-w-sm text-sm leading-relaxed text-saturx-muted">
            {t.footer.tagline}
          </p>
        </div>
        <div>
          <h2 className="text-sm font-bold text-saturx-text">{t.footer.product}</h2>
          <div className="mt-4 grid gap-3 text-sm text-saturx-muted">
            <Link href="#what-is-saturx" className="w-fit rounded-lg hover:text-saturx-text transition-colors">{t.footer.what}</Link>
            <Link href="#features" className="w-fit rounded-lg hover:text-saturx-text transition-colors">{t.footer.features}</Link>
            <Link href="#screenshots" className="w-fit rounded-lg hover:text-saturx-text transition-colors">{t.footer.screenshots}</Link>
            <Link href="#faq" className="w-fit rounded-lg hover:text-saturx-text transition-colors">{t.footer.faq}</Link>
          </div>
        </div>
        <div>
          <h2 className="text-sm font-bold text-saturx-text">{t.footer.getSaturx}</h2>
          <div className="mt-4 grid gap-3 text-sm text-saturx-muted">
            <Link href="#get-saturx" className="w-fit rounded-lg hover:text-saturx-text transition-colors">{t.footer.getSaturx}</Link>
            <a href="https://t.me/saturx_bot" target="_blank" rel="noopener noreferrer" className="w-fit rounded-lg hover:text-saturx-text transition-colors">
              {t.footer.contact}
            </a>
          </div>
        </div>
      </div>
      <p className="section-container mt-10 border-t border-saturx-border pt-6 text-xs leading-relaxed text-saturx-dim">
        (c) {new Date().getFullYear()} SATURX. Kick chat and dashboard desktop tool.
      </p>
    </footer>
  )
}
