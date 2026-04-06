'use client'

import { useLang } from '@/app/context/LangContext'

export default function GetSaturX() {
  const { t } = useLang()

  return (
    <section id="get-saturx" className="py-16 sm:py-20 px-4 sm:px-6 lg:px-8">
      <div className="max-w-2xl mx-auto text-center">
        <h2 className="text-2xl sm:text-3xl font-bold text-saturx-text mb-4">{t.getSaturx.title}</h2>
        <p className="text-saturx-muted mb-10">
          {t.getSaturx.intro}
        </p>
        <ol className="text-left space-y-6 text-saturx-text mb-12">
          <li className="flex gap-4">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-saturx-green text-[#0f0f0f] text-sm font-semibold">1</span>
            <span>{t.getSaturx.step1}</span>
          </li>
          <li className="flex gap-4">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-saturx-green text-[#0f0f0f] text-sm font-semibold">2</span>
            <span>{t.getSaturx.step2}</span>
          </li>
          <li className="flex gap-4">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-saturx-green text-[#0f0f0f] text-sm font-semibold">3</span>
            <span>{t.getSaturx.step3}</span>
          </li>
        </ol>
        <div className="flex flex-wrap justify-center gap-4">
          <a
            href="https://t.me/saturx_bot"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex px-8 py-3.5 rounded-lg bg-saturx-green hover:bg-saturx-green-hover text-[#0f0f0f] font-medium transition-colors"
          >
            {t.getSaturx.telegram}
          </a>
          <a
            href="https://github.com/sssaturX/SaturX-Kick-Ultimate-Tool"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex px-8 py-3.5 rounded-lg border border-saturx-border text-saturx-text hover:bg-saturx-elevated font-medium transition-colors"
          >
            {t.getSaturx.github}
          </a>
        </div>
        <p className="mt-8 text-sm text-saturx-muted">
          {t.getSaturx.footer}
        </p>
      </div>
    </section>
  )
}
