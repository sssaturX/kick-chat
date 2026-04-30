import type { Translation } from '@/app/lib/translations'

export default function Strip({ t }: { t: Translation }) {
  return (
    <section className="soft-divider section-band reveal px-4 py-8 sm:px-6 lg:px-8">
      <p className="mx-auto max-w-2xl text-center text-sm font-medium leading-relaxed text-saturx-muted">
        {t.strip.title}
      </p>
    </section>
  )
}
