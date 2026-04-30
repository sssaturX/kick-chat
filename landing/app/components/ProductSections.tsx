import type { Translation } from '@/app/lib/translations'

export default function ProductSections({ t }: { t: Translation }) {
  return (
    <>
      <section id="what-is-saturx" className="section-divider section-pad reveal px-4 sm:px-6 lg:px-8">
        <div className="section-container">
          <div className="max-w-3xl">
            <h2 className="section-title">{t.product.title}</h2>
            <p className="section-copy mt-4">{t.product.intro}</p>
          </div>

          <div className="stagger mt-10 grid lg:grid-cols-[1fr_0.9fr] gap-6 lg:gap-8">
            <article className="premium-card equal-card reveal border border-saturx-border p-6 sm:p-8">
              <h3 className="card-title">{t.product.whatTitle}</h3>
              <p className="card-copy mt-4">{t.product.whatText}</p>
            </article>

            <article className="premium-card equal-card reveal border border-saturx-border p-6 sm:p-8">
              <h3 className="card-title">{t.product.forTitle}</h3>
              <ul className="mt-5 space-y-4">
                {t.product.forItems.map((item) => (
                  <li key={item} className="card-copy flex gap-3">
                    <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-saturx-green" aria-hidden />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </article>
          </div>
        </div>
      </section>

      <section id="benefits" className="section-divider section-pad section-band reveal px-4 sm:px-6 lg:px-8">
        <div className="section-container">
          <div className="max-w-3xl">
            <h2 className="section-title">{t.product.benefitsTitle}</h2>
            <p className="section-copy mt-4">{t.product.benefitsIntro}</p>
          </div>
          <div className="stagger mt-10 grid md:grid-cols-3 gap-6">
            {t.product.benefits.map((benefit) => (
              <article key={benefit.title} className="premium-card equal-card reveal border border-saturx-border p-6">
                <h3 className="card-title">{benefit.title}</h3>
                <p className="card-copy mt-3">{benefit.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section id="how-it-works" className="section-divider section-pad reveal px-4 sm:px-6 lg:px-8">
        <div className="section-container">
          <div className="max-w-3xl">
            <h2 className="section-title">{t.how.title}</h2>
            <p className="section-copy mt-4">{t.how.intro}</p>
          </div>
          <div className="stagger mt-10 grid lg:grid-cols-3 gap-6">
            {t.how.steps.map((step, index) => (
              <article key={step.title} className="premium-card equal-card reveal border border-saturx-border p-6">
                <span className="flex h-10 w-10 items-center justify-center rounded-full bg-saturx-green text-sm font-bold text-[#0f0f0f]">
                  {index + 1}
                </span>
                <h3 className="card-title mt-5">{step.title}</h3>
                <p className="card-copy mt-3">{step.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section id="why-saturx" className="section-divider section-pad section-band reveal px-4 sm:px-6 lg:px-8">
        <div className="section-container grid lg:grid-cols-[0.8fr_1.2fr] gap-10 lg:gap-14 items-start">
          <div>
            <h2 className="section-title">{t.why.title}</h2>
            <p className="section-copy mt-4">{t.why.intro}</p>
          </div>
          <div className="stagger grid gap-4">
            {t.why.reasons.map((reason) => (
              <article key={reason.title} className="premium-card equal-card reveal border border-saturx-border p-6">
                <h3 className="card-title">{reason.title}</h3>
                <p className="card-copy mt-2">{reason.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section id="trust" className="section-divider section-pad reveal px-4 sm:px-6 lg:px-8">
        <div className="section-container">
          <div className="max-w-3xl">
            <h2 className="section-title">{t.trust.title}</h2>
            <p className="section-copy mt-4">{t.trust.intro}</p>
          </div>
          <div className="stagger mt-10 grid md:grid-cols-3 gap-6">
            {t.trust.items.map((item) => (
              <article key={item.title} className="premium-card equal-card reveal border border-saturx-border p-6">
                <h3 className="card-title">{item.title}</h3>
                <p className="card-copy mt-3">{item.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section id="faq" className="section-divider section-pad section-band reveal px-4 sm:px-6 lg:px-8">
        <div className="max-w-4xl mx-auto">
          <h2 className="section-title">{t.faq.title}</h2>
          <p className="section-copy mt-4">{t.faq.intro}</p>
          <div className="premium-card mt-10 divide-y divide-saturx-border border border-saturx-border overflow-hidden">
            {t.faq.items.map((item) => (
              <details key={item.question} className="faq-item group p-6">
                <summary className="flex cursor-pointer list-none items-center justify-between gap-4 text-base font-semibold leading-snug text-saturx-text">
                  <span>{item.question}</span>
                  <span className="faq-icon text-xl leading-none text-saturx-muted" aria-hidden>+</span>
                </summary>
                <div className="faq-answer">
                  <div>
                    <p className="card-copy mt-3">{item.answer}</p>
                  </div>
                </div>
              </details>
            ))}
          </div>
        </div>
      </section>
    </>
  )
}
