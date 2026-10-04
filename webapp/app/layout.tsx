import type { Metadata } from 'next'
import './globals.css'
import ThemeInit from './theme-init'
import { APPEARANCE_BOOT_SCRIPT } from '@/lib/appearance'

export const metadata: Metadata = {
  title: 'DockLite - Docker Control Panel',
  description: 'Simple Docker-based web hosting control panel',
  icons: {
    icon: '/dockliteiconL.png',
    apple: '/dockliteiconL.png',
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: APPEARANCE_BOOT_SCRIPT }} />
      </head>
      <body>
        <ThemeInit />
        {children}
      </body>
    </html>
  )
}
