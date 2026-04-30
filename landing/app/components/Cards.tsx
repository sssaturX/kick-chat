import Link from 'next/link'
import type { Translation } from '@/app/lib/translations'

export default function Cards({ t }: { t: Translation }) {
  return (
    <section className="section-divider section-pad section-band reveal px-4 sm:px-6 lg:px-8">
      <div className="section-container">
        <div className="stagger grid md:grid-cols-3 gap-6 lg:gap-8">
          {/* Large card — Chat activity */}
          <Link
            href="#features"
            className="premium-card equal-card reveal group block border border-saturx-border p-7 sm:p-10 md:col-span-2"
          >
            <div className="flex flex-col h-full min-h-[220px] sm:min-h-[260px] text-left">
              <span className="inline-flex w-fit text-[11px] font-semibold tracking-wide uppercase text-saturx-green bg-saturx-green-soft px-3 py-1.5 rounded-full mb-6">
                {t.cards.tag}
              </span>
              <h2 className="text-2xl sm:text-3xl font-bold text-saturx-text leading-tight mb-4 group-hover:text-saturx-green transition-colors duration-200">
                {t.cards.mainTitle}
              </h2>
              <p className="section-copy max-w-xl">
                {t.cards.mainDesc}
              </p>
            </div>
          </Link>
          {/* Small cards */}
          <div className="flex flex-col gap-6">
            <Link
              href="#features"
              className="premium-card equal-card reveal block border border-saturx-border p-6"
            >
              <span className="inline-flex text-[11px] font-semibold tracking-wide uppercase text-saturx-green bg-saturx-green-soft px-2.5 py-1 rounded-full mb-3">
                {t.cards.tag}
              </span>
              <h3 className="card-title mb-2">{t.cards.multiTitle}</h3>
              <p className="card-copy">{t.cards.multiDesc}</p>
            </Link>
            <Link
              href="#get-saturx"
              className="premium-card equal-card reveal block border border-saturx-border p-6"
            >
              <span className="inline-flex text-[11px] font-semibold tracking-wide uppercase text-saturx-green bg-saturx-green-soft px-2.5 py-1 rounded-full mb-3">
                {t.cards.tag}
              </span>
              <h3 className="card-title mb-2">{t.cards.licenseTitle}</h3>
              <p className="card-copy">{t.cards.licenseDesc}</p>
            </Link>
          </div>
        </div>
      </div>
    </section>
  )
}
