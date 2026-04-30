import Image from 'next/image'
import type { Lang, Translation } from '@/app/lib/translations'

export default function Hero({ lang, t }: { lang: Lang; t: Translation }) {
  return (
    <section
      id="hero"
      className="hero-scene relative overflow-hidden px-4 sm:px-6 lg:px-8"
    >
      <div className="ambient-hero" aria-hidden />
      <div className="hero-planet" aria-hidden />
      <div className="hero-mountains" aria-hidden />
      <div className="hero-terminal-grid" aria-hidden />
      <div className="hero-scanline" aria-hidden />
      <div className="hero-hud hero-hud-left" aria-hidden />
      <div className="hero-hud hero-hud-right" aria-hidden />
      <div className="hero-grid relative z-10 mx-auto grid min-h-[calc(100svh-4.25rem)] max-w-[82rem] items-center gap-7 py-7 sm:py-8 lg:grid-cols-[0.9fr_1.1fr] lg:gap-8 xl:gap-10">
        <div className="hero-copy max-w-2xl lg:max-w-[39rem]">
          <span className="hero-reveal inline-flex items-center rounded-full border border-saturx-green/30 bg-saturx-green-soft px-3 py-1 text-xs font-semibold uppercase tracking-wide text-saturx-green">
            {t.hero.eyebrow}
          </span>
          <HeroTitle lang={lang} headline={t.hero.headline} />
          <p className="hero-subheadline hero-reveal hero-delay-2 mt-4 max-w-xl text-base leading-relaxed text-saturx-muted sm:text-lg">
            {t.hero.subheadline}
          </p>
          <p className="hero-desc hero-reveal hero-delay-2 mt-3 max-w-lg text-sm leading-relaxed text-saturx-dim before:mr-2 before:inline-block before:h-2 before:w-2 before:rounded-full before:bg-saturx-green before:shadow-[0_0_14px_rgba(83,252,24,0.8)] sm:text-base">
            {t.hero.desc}
          </p>
          <div className="hero-actions hero-reveal hero-delay-3 mt-5 flex flex-col gap-2.5 sm:flex-row sm:flex-wrap">
            <a
              href="#get-saturx"
              className="premium-button primary-glow inline-flex min-h-12 items-center justify-center gap-3 rounded-lg bg-saturx-green px-6 py-3 font-bold text-[#0f0f0f] hover:bg-saturx-green-hover"
            >
              <DownloadIcon />
              <span className="relative z-10">{t.hero.button}</span>
            </a>
            <a
              href="#features"
              className="premium-button inline-flex min-h-12 items-center justify-center gap-3 rounded-lg border border-saturx-border px-6 py-3 font-bold text-saturx-text hover:bg-saturx-elevated"
            >
              <span className="relative z-10">{t.hero.learnMore}</span>
              <ArrowIcon />
            </a>
          </div>
          <div className="hero-proof hero-reveal hero-delay-4 mt-5 grid max-w-xl gap-2 sm:grid-cols-3">
            {t.hero.proof.map((item, index) => (
              <span key={item} className="premium-chip flex min-h-10 items-center gap-2.5 rounded-lg border border-saturx-border bg-saturx-panel/70 px-3 py-2 text-xs font-semibold text-saturx-text lg:text-[0.8rem]">
                <ProofIcon index={index} />
                {item}
              </span>
            ))}
          </div>
        </div>
        <div className="hero-preview-wrap hero-reveal hero-delay-3 relative">
          <HeroPreview t={t} />
        </div>
      </div>
    </section>
  )
}

function HeroTitle({ lang, headline }: { lang: Lang; headline: string }) {
  if (lang === 'en') {
    return (
      <h1 className="hero-title hero-reveal hero-delay-1 mt-4 font-bold tracking-tight text-saturx-text">
        SATURX brings
        <br />
        <span className="hero-highlight">Kick</span> chat,
        <br />
        <span className="hero-highlight">dashboard</span> activity,
        <br />
        and <span className="hero-highlight">viewer</span> controls
        <br />
        into one workspace.
      </h1>
    )
  }

  return (
    <h1 className="hero-title hero-reveal hero-delay-1 mt-4 font-bold tracking-tight text-saturx-text">
      SATURX объединяет
      <br />
      <span className="hero-highlight">Kick-чат</span>, дашборд
      <br />
      и контроль активности
      <br />
      в одном окне.
    </h1>
  )
}

function HeroPreview({ t }: { t: Translation }) {
  return (
    <div className="hero-preview-card mockup-frame relative rounded-xl border border-saturx-border bg-saturx-elevated/80 p-2.5 shadow-[0_22px_70px_rgba(0,0,0,0.42)]">
      <div className="hero-preview-head flex items-center justify-between border-b border-saturx-border px-2 pb-2.5">
        <div>
          <p className="text-sm font-semibold text-saturx-text">{t.hero.visualTitle}</p>
          <p className="text-xs text-saturx-muted">{t.hero.visualSubtitle}</p>
        </div>
        <span className="rounded-full bg-saturx-green px-2.5 py-1 text-xs font-bold text-[#0f0f0f]">SATURX</span>
      </div>
      <div className="hero-preview-media relative mt-2.5 aspect-[16/9] overflow-hidden rounded-lg bg-saturx-bg">
        <Image
          src="/img/dash.png"
          alt={t.screenshots.dashboardAlt}
          fill
          priority
          className="mockup-image object-cover object-top"
          sizes="(max-width: 1024px) 100vw, 720px"
        />
      </div>
      <div className="hero-preview-badges mt-2.5 grid grid-cols-2 gap-1.5 sm:grid-cols-4">
        {t.hero.visualBadges.map((badge) => (
          <span key={badge} className="rounded-md border border-saturx-border bg-saturx-panel px-2 py-1.5 text-center text-xs font-medium text-saturx-muted">
            {badge}
          </span>
        ))}
      </div>
    </div>
  )
}

function DownloadIcon() {
  return (
    <svg className="relative z-10 h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M12 3v11" />
      <path d="m7 10 5 5 5-5" />
      <path d="M5 21h14" />
    </svg>
  )
}

function ArrowIcon() {
  return (
    <svg className="relative z-10 h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M5 12h14" />
      <path d="m13 6 6 6-6 6" />
    </svg>
  )
}

function ProofIcon({ index }: { index: number }) {
  const paths = [
    <path key="bolt" d="m13 2-8 12h6l-1 8 9-13h-6l0-7Z" />,
    <path key="shield" d="M12 3 5 6v5c0 5 3.5 8 7 10 3.5-2 7-5 7-10V6l-7-3Z" />,
    <path key="chart" d="M4 17h16M6 14l4-4 3 3 5-7" />,
  ]

  return (
    <svg className="h-5 w-5 shrink-0 text-saturx-green drop-shadow-[0_0_10px_rgba(83,252,24,0.55)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      {paths[index] ?? paths[0]}
    </svg>
  )
}
