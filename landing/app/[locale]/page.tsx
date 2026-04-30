import Header from '@/app/components/Header'
import Hero from '@/app/components/Hero'
import Strip from '@/app/components/Strip'
import Cards from '@/app/components/Cards'
import ProductSections from '@/app/components/ProductSections'
import Features from '@/app/components/Features'
import Screenshots from '@/app/components/Screenshots'
import GetSaturX from '@/app/components/GetSaturX'
import Footer from '@/app/components/Footer'
import CryptoCursor from '@/app/components/CryptoCursor'
import { notFound } from 'next/navigation'
import type { Lang } from '@/app/lib/translations'
import { translations } from '@/app/lib/translations'

const locales = ['en', 'ru'] as const

function isLocale(locale: string): locale is Lang {
  return locales.includes(locale as Lang)
}

export default function LocaleHome({ params }: { params: { locale: string } }) {
  if (!isLocale(params.locale)) notFound()

  const lang = params.locale
  const t = translations[lang]

  return (
    <div className="page-shell min-h-screen w-full">
      <CryptoCursor />
      <div className="h-1 bg-saturx-green" aria-hidden />
      <Header lang={lang} copy={{ nav: t.nav, cta: t.cta }} />
      <main>
        <Hero lang={lang} t={t} />
        <Strip t={t} />
        <ProductSections t={t} />
        <Cards t={t} />
        <Features t={t} />
        <Screenshots t={t} />
        <GetSaturX t={t} />
      </main>
      <Footer t={t} />
    </div>
  )
}
