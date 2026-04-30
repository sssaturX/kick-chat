import Image from 'next/image'
import type { Translation } from '@/app/lib/translations'

export default function Screenshots({ t }: { t: Translation }) {
  const screens = [
    { title: t.screenshots.dashboardTitle, description: t.screenshots.dashboardDesc, src: '/img/dash.png', alt: t.screenshots.dashboardAlt },
    { title: t.screenshots.viewerbotTitle, description: t.screenshots.viewerbotDesc, src: '/img/viewer.png', alt: t.screenshots.viewerbotAlt },
  ]

  return (
    <section id="screenshots" className="section-divider section-pad section-band reveal px-4 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-5xl">
        <h2 className="section-title mb-3">{t.screenshots.title}</h2>
        <p className="section-copy mb-12 max-w-2xl">
          {t.screenshots.intro}
        </p>
        <div className="stagger space-y-8">
          {screens.map(({ title, description, src, alt }, i) => (
            <div key={i} className="showcase-card premium-card reveal overflow-hidden border border-saturx-border shadow-[0_20px_70px_rgba(0,0,0,0.25)]">
              <div className="relative aspect-video w-full bg-saturx-bg">
                <Image
                  src={src}
                  alt={alt}
                  fill
                  className="mockup-image object-contain object-top"
                  sizes="(max-width: 1024px) 100vw, 1024px"
                />
              </div>
              <div className="p-6 sm:p-7 border-t border-saturx-border">
                <h3 className="card-title">{title}</h3>
                <p className="card-copy mt-2">{description}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
