import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import { LangProvider } from './context/LangContext'
import './globals.css'

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
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
    <html lang="en" className={inter.variable}>
      <body className="font-sans antialiased">
        <LangProvider>{children}</LangProvider>
      </body>
    </html>
  )
}
