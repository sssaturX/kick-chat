'use client'

import { useLang } from '@/app/context/LangContext'

export default function GetSaturX() {
  const { t } = useLang()

  return (
    <section id="get-saturx" className="py-16 sm:py-20 px-4 sm:px-6 lg:px-8">
      <div className="max-w-2xl mx-auto text-center">
        <h2 className="text-2xl sm:text-3xl font-bold text-saturx-dark mb-4">{t.getSaturx.title}</h2>
        <p className="text-saturx-muted mb-10">
          {t.getSaturx.intro}
        </p>
        <ol className="text-left space-y-6 text-saturx-dark mb-12">
          <li className="flex gap-4">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-saturx-purple text-white text-sm font-semibold">1</span>
            <span>{t.getSaturx.step1}</span>
          </li>
          <li className="flex gap-4">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-saturx-purple text-white text-sm font-semibold">2</span>
            <span>{t.getSaturx.step2}</span>
          </li>
          <li className="flex gap-4">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-saturx-purple text-white text-sm font-semibold">3</span>
            <span>{t.getSaturx.step3}</span>
          </li>
        </ol>
        <div className="flex flex-wrap justify-center gap-4">
          <a
            href="https://t.me/pump1sss"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex px-8 py-3.5 rounded-lg bg-saturx-purple hover:bg-saturx-purple-light text-white font-medium transition-colors"
          >
            {t.getSaturx.telegram}
          </a>
          <a
            href="https://github.com/santoridev"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex px-8 py-3.5 rounded-lg border border-saturx-border text-saturx-dark hover:bg-saturx-bg-subtle font-medium transition-colors"
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
