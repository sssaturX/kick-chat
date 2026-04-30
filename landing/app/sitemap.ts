import type { MetadataRoute } from 'next'

const siteUrl = 'https://saturx.store'

export default function sitemap(): MetadataRoute.Sitemap {
  const languages = {
    en: `${siteUrl}/en`,
    ru: `${siteUrl}/ru`,
    'x-default': `${siteUrl}/en`,
  }

  return [
    {
      url: `${siteUrl}/en`,
      lastModified: new Date(),
      changeFrequency: 'weekly',
      priority: 1,
      alternates: {
        languages,
      },
    },
    {
      url: `${siteUrl}/ru`,
      lastModified: new Date(),
      changeFrequency: 'weekly',
      priority: 0.95,
      alternates: {
        languages,
      },
    },
  ]
}
