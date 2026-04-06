'use client'

import { useLang } from '@/app/context/LangContext'

export default function Strip() {
  const { t } = useLang()

  return (
    <section className="border-t border-saturx-border py-8 px-4 sm:px-6 lg:px-8 bg-saturx-panel/50">
      <p className="text-center text-sm text-saturx-muted max-w-2xl mx-auto">
        {t.strip.title}
      </p>
    </section>
  )
}
