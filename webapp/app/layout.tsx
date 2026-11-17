import type { Metadata } from 'next'
import localFont from 'next/font/local'
import './globals.css'
import ThemeInit from './theme-init'
import { APPEARANCE_BOOT_SCRIPT } from '@/lib/appearance'
import { DEMO } from '@/lib/demo/flag'

// Only present in demo builds: the require is behind the build-time flag, so normal builds drop it entirely.
const DemoBoot = process.env.NEXT_PUBLIC_DOCKLITE_DEMO === '1' ? require('@/lib/demo/DemoBoot').default : null

// Fonts are bundled with the app (open-licence files in app/fonts, licences in app/fonts/licenses), so a server never
// fetches them from anywhere and every theme looks the same on every machine.
//   Chakra Petch   squared, techno: the Cyberpunk look
//   Inter          clean business sans: Corpo and Corpo Blue
//   Nunito         soft and round: Unicorn
//   JetBrains Mono ports, paths, code and numbers, in every theme
const chakra = localFont({
  src: [
    { path: './fonts/chakra-petch-latin-400-normal.woff2', weight: '400', style: 'normal' },
    { path: './fonts/chakra-petch-latin-500-normal.woff2', weight: '500', style: 'normal' },
    { path: './fonts/chakra-petch-latin-600-normal.woff2', weight: '600', style: 'normal' },
    { path: './fonts/chakra-petch-latin-700-normal.woff2', weight: '700', style: 'normal' },
  ],
  variable: '--font-display',
  display: 'swap',
})
const inter = localFont({ src: './fonts/inter-latin-wght-normal.woff2', weight: '100 900', variable: '--font-sans', display: 'swap' })
const nunito = localFont({ src: './fonts/nunito-latin-wght-normal.woff2', weight: '200 1000', variable: '--font-round', display: 'swap' })
const jetbrains = localFont({ src: './fonts/jetbrains-mono-latin-wght-normal.woff2', weight: '100 800', variable: '--font-code', display: 'swap' })

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
    <html lang="en" suppressHydrationWarning className={`${chakra.variable} ${inter.variable} ${nunito.variable} ${jetbrains.variable}`}>
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
