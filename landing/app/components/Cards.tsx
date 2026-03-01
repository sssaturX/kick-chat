'use client'

import Link from 'next/link'
import { useLang } from '@/app/context/LangContext'

export default function Cards() {
  const { t } = useLang()

  return (
    <section className="py-12 sm:py-16 lg:py-20 px-4 sm:px-6 lg:px-8 bg-saturx-bg-subtle">
      <div className="max-w-6xl mx-auto">
        <div className="grid md:grid-cols-3 gap-6 lg:gap-8">
          {/* Large card — Chat activity */}
          <Link
            href="#features"
            className="md:col-span-2 group block rounded-2xl bg-white p-8 sm:p-10 shadow-[0_1px_3px_rgba(0,0,0,0.06),0_4px_12px_rgba(0,0,0,0.04)] hover:shadow-[0_4px_20px_rgba(91,33,182,0.08),0_2px_8px_rgba(0,0,0,0.06)] border border-saturx-border/80 hover:border-saturx-purple/20 transition-all duration-300"
          >
            <div className="flex flex-col h-full min-h-[220px] sm:min-h-[260px] text-left">
              <span className="inline-flex w-fit text-[11px] font-semibold tracking-wide uppercase text-saturx-purple bg-[#ebe8f3] px-3 py-1.5 rounded-full mb-6">
                {t.cards.tag}
              </span>
              <h2 className="text-2xl sm:text-3xl font-bold text-[#1f2937] leading-tight mb-4 group-hover:text-saturx-purple transition-colors duration-200">
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
              className="rounded-2xl bg-white p-6 shadow-[0_1px_3px_rgba(0,0,0,0.06)] hover:shadow-[0_4px_14px_rgba(0,0,0,0.08)] border border-saturx-border/80 hover:border-saturx-purple/20 transition-all duration-300"
            >
              <span className="inline-flex text-[11px] font-semibold tracking-wide uppercase text-saturx-purple bg-[#ebe8f3] px-2.5 py-1 rounded-full mb-3">
                {t.cards.tag}
              </span>
              <h3 className="text-lg font-bold text-saturx-dark mb-2">{t.cards.multiTitle}</h3>
              <p className="text-sm text-saturx-muted leading-relaxed">{t.cards.multiDesc}</p>
            </Link>
            <Link
              href="#get-saturx"
              className="rounded-2xl bg-white p-6 shadow-[0_1px_3px_rgba(0,0,0,0.06)] hover:shadow-[0_4px_14px_rgba(0,0,0,0.08)] border border-saturx-border/80 hover:border-saturx-purple/20 transition-all duration-300"
            >
              <span className="inline-flex text-[11px] font-semibold tracking-wide uppercase text-saturx-purple bg-[#ebe8f3] px-2.5 py-1 rounded-full mb-3">
                {t.cards.tag}
              </span>
              <h3 className="text-lg font-bold text-saturx-dark mb-2">{t.cards.licenseTitle}</h3>
              <p className="text-sm text-saturx-muted leading-relaxed">{t.cards.licenseDesc}</p>
            </Link>
          </div>
        </div>
      </div>
    </section>
  )
}
