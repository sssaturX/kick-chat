# SaturX landing page

Marketing landing page for selling SaturX (Next.js). Dark theme, purple gradient, frame layout.

## Run

```bash
cd landing
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

## Build

```bash
npm run build
npm run start
```

## Customize

- **Contact link:** In `app/components/GetSaturX.tsx` replace `href="https://github.com"` with your repo URL, Telegram, or contact page.
- **Screenshots:** Add real app screenshots in `app/components/Screenshots.tsx` (use `<Image>` or `<img src="/screenshot-dashboard.png" />` and put images in `public/`).
- **Logo:** The logo in `app/components/Logo.tsx` is an SVG (arc + dot). To use your own PNG, replace the `<svg>` with `<img src="/logo.png" alt="SaturX" />` and add `logo.png` to `public/`.
