import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import { notFound } from 'next/navigation'
import type { Lang } from '@/app/lib/translations'
import { translations } from '@/app/lib/translations'
import '@/app/globals.css'

const siteUrl = 'https://saturx.store'
const siteName = 'SATURX'
const locales = ['en', 'ru'] as const
const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
})

type Locale = (typeof locales)[number]

function isLocale(locale: string): locale is Locale {
  return locales.includes(locale as Locale)
}

function getDescription(locale: Locale) {
  const t = translations[locale]
  return `${t.product.intro} ${t.hero.subheadline}`
}

function getTitle(locale: Locale) {
  return locale === 'ru'
    ? 'SATURX - инструмент для Kick-чата и дашборда'
    : 'SATURX - Kick chat and dashboard tool'
}

function getJsonLd(locale: Locale) {
  const description = getDescription(locale)
  const pageUrl = `${siteUrl}/${locale}`

  return [
    {
      '@context': 'https://schema.org',
      '@type': 'Organization',
      '@id': `${siteUrl}/#organization`,
      name: siteName,
      url: siteUrl,
      logo: `${siteUrl}/icon.svg`,
      sameAs: ['https://t.me/saturx_bot', 'https://github.com/sssaturX/SaturX-Kick-Ultimate-Tool'],
    },
    {
      '@context': 'https://schema.org',
      '@type': 'WebSite',
      '@id': `${siteUrl}/#website-${locale}`,
      name: siteName,
      url: pageUrl,
      description,
      inLanguage: locale,
      publisher: {
        '@id': `${siteUrl}/#organization`,
      },
    },
    {
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      '@id': `${siteUrl}/#software-${locale}`,
      name: siteName,
      url: pageUrl,
      description,
      inLanguage: locale,
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Windows',
      softwareHelp: 'https://t.me/saturx_bot',
      image: `${siteUrl}/img/dash.png`,
      publisher: {
        '@id': `${siteUrl}/#organization`,
      },
    },
  ]
}

export function generateStaticParams() {
  return locales.map((locale) => ({ locale }))
}

export function generateMetadata({
  params,
}: {
  params: { locale: string }
}): Metadata {
  if (!isLocale(params.locale)) notFound()

  const locale = params.locale
  const title = getTitle(locale)
  const description = getDescription(locale)
  const url = `${siteUrl}/${locale}`

  return {
    metadataBase: new URL(siteUrl),
    title: {
      default: title,
      template: '%s | SATURX',
    },
    description,
    keywords: [
      'SATURX',
      'Kick chat tool',
      'Kick dashboard',
      'Kick streamer tool',
      'Kick moderation tool',
      'desktop streaming tool',
      'multi account chat',
      'viewer boost controls',
      'инструмент для Kick',
      'дашборд Kick',
    ],
    alternates: {
      canonical: `/${locale}`,
      languages: {
        en: '/en',
        ru: '/ru',
        'x-default': '/en',
      },
    },
    openGraph: {
      type: 'website',
      url,
      siteName,
      title,
      description,
      locale,
      alternateLocale: locale === 'en' ? ['ru'] : ['en'],
      images: [
        {
          url: '/img/dash.png',
          width: 1716,
          height: 922,
          alt: translations[locale].screenshots.dashboardAlt,
        },
      ],
    },
    twitter: {
      card: 'summary_large_image',
      title,
      description,
      images: ['/img/dash.png'],
    },
    robots: {
      index: true,
      follow: true,
      googleBot: {
        index: true,
        follow: true,
        'max-image-preview': 'large',
        'max-snippet': -1,
        'max-video-preview': -1,
      },
    },
    icons: {
      icon: '/icon.svg',
      shortcut: '/icon.svg',
      apple: '/apple-icon.svg',
    },
    manifest: '/manifest.webmanifest',
    applicationName: siteName,
    creator: siteName,
    publisher: siteName,
  }
}

export default function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode
  params: { locale: string }
}) {
  if (!isLocale(params.locale)) notFound()

  const locale = params.locale as Lang

  return (
    <html lang={locale} className={inter.variable}>
      <body className="font-sans antialiased">
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(getJsonLd(locale)) }}
        />
        {children}
      </body>
    </html>
  )
}
