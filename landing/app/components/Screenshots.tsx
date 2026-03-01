'use client'

import Image from 'next/image'
import { useLang } from '@/app/context/LangContext'

export default function Screenshots() {
  const { t } = useLang()

  const screens = [
    { title: t.screenshots.dashboardTitle, description: t.screenshots.dashboardDesc, src: '/img/dash.png', alt: 'SaturX Dashboard' },
    { title: t.screenshots.viewerbotTitle, description: t.screenshots.viewerbotDesc, src: '/img/viewer.png', alt: 'SaturX Viewer boost' },
  ]

  return (
    <section id="screenshots" className="py-16 sm:py-20 px-4 sm:px-6 lg:px-8 bg-saturx-bg-subtle">
      <div className="max-w-5xl mx-auto">
        <h2 className="text-2xl sm:text-3xl font-bold text-saturx-dark mb-3">{t.screenshots.title}</h2>
        <p className="text-saturx-muted mb-10 max-w-2xl">
          {t.screenshots.intro}
        </p>
        <div className="space-y-8">
          {screens.map(({ title, description, src, alt }, i) => (
            <div key={i} className="rounded-2xl border border-saturx-border bg-white overflow-hidden shadow-sm">
              <div className="relative aspect-video w-full bg-saturx-bg-subtle">
                <Image
                  src={src}
                  alt={alt}
                  fill
                  className="object-contain object-top"
                  sizes="(max-width: 1024px) 100vw, 1024px"
                />
              </div>
              <div className="p-6 border-t border-saturx-border">
                <h3 className="text-lg font-semibold text-saturx-dark">{title}</h3>
                <p className="mt-2 text-sm text-saturx-muted">{description}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
