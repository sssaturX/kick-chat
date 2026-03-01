import type { Metadata } from 'next'
import { Outfit } from 'next/font/google'
import { LangProvider } from './context/LangContext'
import './globals.css'

const outfit = Outfit({
  subsets: ['latin'],
  variable: '--font-outfit',
})

export const metadata: Metadata = {
  title: 'SaturX — Kick chat & dashboard in one window',
  description: 'One interface. Multiple accounts. Your Kick channel under control. Desktop app for streamers and moderators.',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className={outfit.variable}>
      <body className="font-sans antialiased">
        <LangProvider>{children}</LangProvider>
      </body>
    </html>
  )
}
