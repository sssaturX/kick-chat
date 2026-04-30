# SATURX landing page

Marketing landing page for SATURX, a desktop Kick chat and dashboard tool for streamers and moderators.

## Stack

- Next.js 14 App Router
- React 18
- TypeScript
- Tailwind CSS

## Run

```bash
cd landing
npm install
npm run dev
```

Open [http://localhost:3000/en](http://localhost:3000/en). The root path redirects to `/en`.

## Build

```bash
npm run build
npm run start
```

## SEO

The landing includes localized Next metadata, canonical URLs, hreflang alternates, Open Graph metadata, Twitter Card metadata, robots, sitemap, manifest, favicon SVG, and JSON-LD for Organization, WebSite, and SoftwareApplication.

Primary domain: `https://saturx.store`

## Content

Text is localized in `app/lib/translations.ts` for English and Russian. Public localized routes are `/en` and `/ru`; `/` redirects to `/en`.

## Assets

Product screenshots live in `public/img/`.
