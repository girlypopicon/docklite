import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      screens: {
        '3xl': '2560px',
      },
      gridTemplateColumns: {
        '8': 'repeat(8, minmax(0, 1fr))',
      },
      colors: {
        'dark-bg': 'var(--bg-dark)',
        'neon-cyan': 'var(--neon-cyan)',
        'neon-pink': 'var(--neon-pink)',
        'neon-purple': 'var(--neon-purple)',
        'neon-green': 'var(--neon-green)',
        'neon-yellow': 'var(--neon-yellow)',
        // text colours: readable on every theme (see --text-* in globals.css)
        'accent-cyan': 'var(--text-cyan)',
        'accent-pink': 'var(--text-pink)',
        'accent-purple': 'var(--text-purple)',
        'accent-green': 'var(--text-green)',
        'accent-yellow': 'var(--text-yellow)',
        'ink-muted': 'var(--text-muted)',
        'ink-secondary': 'var(--text-secondary)',
        'surface-muted': 'var(--surface-muted)',
        'status-error': 'var(--status-error)',
        'status-warning': 'var(--status-warning)',
        'status-success': 'var(--status-success)',
        'status-info': 'var(--status-info)',
      }
    },
  },
  plugins: [],
}
export default config
