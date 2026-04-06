'use client'

import Link from 'next/link'
import { useLang } from '@/app/context/LangContext'

export default function Cards() {
  const { t } = useLang()

  return (
    <section className="py-12 sm:py-16 lg:py-20 px-4 sm:px-6 lg:px-8 bg-saturx-panel/60">
      <div className="max-w-6xl mx-auto">
        <div className="grid md:grid-cols-3 gap-6 lg:gap-8">
          {/* Large card — Chat activity */}
          <Link
            href="#features"
            className="md:col-span-2 group block rounded-2xl bg-saturx-elevated p-8 sm:p-10 border border-saturx-border hover:border-saturx-green/30 hover:shadow-[0_0_0_1px_rgba(83,252,24,0.15)] transition-all duration-300"
          >
            <div className="flex flex-col h-full min-h-[220px] sm:min-h-[260px] text-left">
              <span className="inline-flex w-fit text-[11px] font-semibold tracking-wide uppercase text-saturx-green bg-saturx-green-soft px-3 py-1.5 rounded-full mb-6">
                {t.cards.tag}
              </span>
              <h2 className="text-2xl sm:text-3xl font-bold text-saturx-text leading-tight mb-4 group-hover:text-saturx-green transition-colors duration-200">
                {t.cards.mainTitle}
              </h2>
              <p className="text-saturx-muted text-base sm:text-lg leading-relaxed max-w-xl">
                {t.cards.mainDesc}
              </p>
            </div>
          </Link>
          {/* Small cards */}
          <div className="flex flex-col gap-6">
            <Link
              href="#features"
              className="rounded-2xl bg-saturx-elevated p-6 border border-saturx-border hover:border-saturx-green/30 hover:shadow-[0_0_0_1px_rgba(83,252,24,0.1)] transition-all duration-300"
            >
              <span className="inline-flex text-[11px] font-semibold tracking-wide uppercase text-saturx-green bg-saturx-green-soft px-2.5 py-1 rounded-full mb-3">
                {t.cards.tag}
              </span>
              <h3 className="text-lg font-bold text-saturx-text mb-2">{t.cards.multiTitle}</h3>
              <p className="text-sm text-saturx-muted leading-relaxed">{t.cards.multiDesc}</p>
            </Link>
            <Link
              href="#get-saturx"
              className="rounded-2xl bg-saturx-elevated p-6 border border-saturx-border hover:border-saturx-green/30 hover:shadow-[0_0_0_1px_rgba(83,252,24,0.1)] transition-all duration-300"
            >
              <span className="inline-flex text-[11px] font-semibold tracking-wide uppercase text-saturx-green bg-saturx-green-soft px-2.5 py-1 rounded-full mb-3">
                {t.cards.tag}
              </span>
              <h3 className="text-lg font-bold text-saturx-text mb-2">{t.cards.licenseTitle}</h3>
              <p className="text-sm text-saturx-muted leading-relaxed">{t.cards.licenseDesc}</p>
            </Link>
          </div>
        </div>
      </div>
    </section>
  )
}
