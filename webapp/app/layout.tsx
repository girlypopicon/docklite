import type { Metadata } from 'next'
import './globals.css'
import ThemeInit from './theme-init'
import { APPEARANCE_BOOT_SCRIPT } from '@/lib/appearance'
import { DEMO } from '@/lib/demo/flag'

// Only present in demo builds: the require is behind the build-time flag, so normal builds drop it entirely.
const DemoBoot = process.env.NEXT_PUBLIC_DOCKLITE_DEMO === '1' ? require('@/lib/demo/DemoBoot').default : null

export const metadata: Metadata = {
  title: DEMO ? 'DockLite demo' : 'DockLite - Docker Control Panel',
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
        {DemoBoot && <DemoBoot />}
        {children}
      </body>
    </html>
  )
}
