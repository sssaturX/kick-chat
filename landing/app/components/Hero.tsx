'use client'

import { useLang } from '@/app/context/LangContext'

export default function Hero() {
  const { t } = useLang()

  return (
    <section
      id="hero"
      className="relative py-12 sm:py-16 lg:py-24 px-4 sm:px-6 lg:px-8 max-w-7xl mx-auto"
    >
      <div className="grid lg:grid-cols-2 gap-10 lg:gap-16 items-center">
        <div>
          <h1 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-saturx-text tracking-tight">
            {t.hero.headline}
          </h1>
          <p className="mt-6 text-lg text-saturx-muted max-w-xl">
            {t.hero.desc}
          </p>
          <div className="mt-8 flex flex-wrap gap-4">
            <a
              href="#get-saturx"
              className="inline-flex px-6 py-3.5 rounded-lg bg-saturx-green hover:bg-saturx-green-hover text-[#0f0f0f] font-medium transition-colors"
            >
              {t.hero.button}
            </a>
            <a
              href="#features"
              className="inline-flex px-6 py-3.5 rounded-lg border border-saturx-border text-saturx-text hover:bg-saturx-elevated font-medium transition-colors"
            >
              {t.hero.learnMore}
            </a>
          </div>
        </div>
        <div className="relative flex justify-center lg:justify-end">
          <HeroGraphic />
        </div>
      </div>
    </section>
  )
}

function HeroGraphic() {
  return (
    <div className="relative w-full max-w-md aspect-square">
      <svg viewBox="0 0 400 400" className="w-full h-full" aria-hidden>
        <defs>
          <linearGradient id="hero-grad1" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#53fc18" />
            <stop offset="100%" stopColor="#6fff2e" />
          </linearGradient>
          <linearGradient id="hero-grad2" x1="100%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stopColor="#2d2d32" />
            <stop offset="100%" stopColor="#53fc18" />
          </linearGradient>
          <linearGradient id="hero-grad3" x1="0%" y1="100%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="#1f1f23" />
            <stop offset="100%" stopColor="#53fc18" />
          </linearGradient>
        </defs>
        <path d="M80 80 L320 80 L320 320 L80 320 Z" fill="none" stroke="url(#hero-grad1)" strokeWidth="2" strokeDasharray="8 6" opacity="0.8" />
        <path d="M120 120 L280 120 L280 280 L120 280 Z" fill="none" stroke="url(#hero-grad2)" strokeWidth="1.5" strokeDasharray="6 4" opacity="0.7" />
        <path d="M160 160 L240 160 L240 240 L160 240 Z" fill="none" stroke="url(#hero-grad3)" strokeWidth="2" opacity="0.9" />
        <circle cx="200" cy="200" r="40" fill="none" stroke="url(#hero-grad1)" strokeWidth="2" strokeDasharray="4 8" opacity="0.6" />
        <circle cx="200" cy="200" r="24" fill="url(#hero-grad1)" fillOpacity="0.15" stroke="url(#hero-grad2)" strokeWidth="1.5" />
        {[60, 140, 260, 340].map((x, i) => (
          <circle key={i} cx={x} cy={200} r="6" fill="url(#hero-grad1)" opacity="0.5" />
        ))}
        {[60, 140, 260, 340].map((y, i) => (
          <circle key={i} cx={200} cy={y} r="4" fill="url(#hero-grad2)" opacity="0.5" />
        ))}
      </svg>
    </div>
  )
}
