'use client'

import { useLang } from '@/app/context/LangContext'

function IconOne() {
  return (
    <svg className="w-10 h-10 text-saturx-green" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
    </svg>
  )
}

function IconTwo() {
  return (
    <svg className="w-10 h-10 text-saturx-green" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
    </svg>
  )
}

function IconThree() {
  return (
    <svg className="w-10 h-10 text-saturx-green" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
    </svg>
  )
}

const icons = [IconOne, IconTwo, IconThree]

export default function Features() {
  const { t } = useLang()
  const blocks = [
    { title: t.featuresBlock.one.title, desc: t.featuresBlock.one.desc },
    { title: t.featuresBlock.two.title, desc: t.featuresBlock.two.desc },
    { title: t.featuresBlock.three.title, desc: t.featuresBlock.three.desc },
  ]

  return (
    <section id="features" className="py-16 sm:py-20 lg:py-24 px-4 sm:px-6 lg:px-8">
      <div className="max-w-6xl mx-auto">
        <h2 className="text-2xl sm:text-3xl lg:text-4xl font-bold text-saturx-text text-center max-w-2xl mx-auto">
          {t.featuresBlock.title}
        </h2>
        <p className="mt-4 text-saturx-muted text-center max-w-xl mx-auto">
          {t.featuresBlock.intro}
        </p>
        <div className="mt-12 sm:mt-16 grid sm:grid-cols-2 lg:grid-cols-3 gap-8 lg:gap-10">
          {blocks.map((block, i) => {
            const Icon = icons[i]
            return (
              <div
                key={i}
                className="rounded-2xl border border-saturx-border bg-saturx-panel p-6 sm:p-8 hover:border-saturx-green/30 hover:shadow-[0_0_0_1px_rgba(83,252,24,0.1)] transition-all"
              >
                <div className="flex items-center justify-center w-14 h-14 rounded-xl border-2 border-saturx-green/30 bg-saturx-green-soft mb-5">
                  <Icon />
                </div>
                <h3 className="text-lg font-semibold text-saturx-text mb-2">{block.title}</h3>
                <p className="text-sm text-saturx-muted">{block.desc}</p>
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}
