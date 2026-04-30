import type { Translation } from '@/app/lib/translations'

export default function GetSaturX({ t }: { t: Translation }) {
  const trustCards = [
    { title: t.getSaturx.accessTitle, text: t.getSaturx.accessText },
    { title: t.getSaturx.supportTitle, text: t.getSaturx.supportText },
    { title: t.getSaturx.safetyTitle, text: t.getSaturx.safetyText },
    { title: t.getSaturx.updatesTitle, text: t.getSaturx.updatesText },
  ]

  return (
    <section id="get-saturx" className="section-divider section-pad reveal px-4 sm:px-6 lg:px-8">
      <div className="premium-card section-container border border-saturx-green/20 p-6 shadow-[0_24px_90px_rgba(0,0,0,0.34)] sm:p-8 lg:p-10">
        <div className="grid lg:grid-cols-[0.9fr_1.1fr] gap-10 lg:gap-14">
          <div>
            <h2 className="section-title mb-4">{t.getSaturx.title}</h2>
            <p className="section-copy">
              {t.getSaturx.intro}
            </p>
            <div className="mt-8">
              <h3 className="card-title mb-4">{t.getSaturx.includesTitle}</h3>
              <ul className="space-y-3 text-sm leading-relaxed text-saturx-muted">
                {t.getSaturx.includes.map((item) => (
                  <li key={item} className="flex gap-3">
                    <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-saturx-green" aria-hidden />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="mt-8 flex flex-wrap gap-4">
              <a
                href="https://t.me/saturx_bot"
                target="_blank"
                rel="noopener noreferrer"
                className="premium-button primary-glow inline-flex min-h-12 items-center justify-center rounded-lg bg-saturx-green px-8 py-3.5 font-bold text-[#0f0f0f] hover:bg-saturx-green-hover"
              >
                <span className="relative z-10">{t.getSaturx.telegram}</span>
              </a>
              <a
                href="https://github.com/sssaturX/SaturX-Kick-Ultimate-Tool"
                target="_blank"
                rel="noopener noreferrer"
                className="premium-button inline-flex min-h-12 items-center justify-center rounded-lg border border-saturx-border px-8 py-3.5 font-bold text-saturx-text hover:bg-saturx-panel"
              >
                <span className="relative z-10">{t.getSaturx.github}</span>
              </a>
            </div>
            <p className="mt-6 text-sm leading-relaxed text-saturx-muted">
              {t.getSaturx.footer}
            </p>
          </div>
          <div className="stagger grid sm:grid-cols-2 gap-4">
            {trustCards.map((card) => (
              <div key={card.title} className="premium-card equal-card reveal border border-saturx-border p-5">
                <h3 className="card-title text-base">{card.title}</h3>
                <p className="card-copy mt-3">{card.text}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
