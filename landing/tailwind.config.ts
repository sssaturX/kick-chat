import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        saturx: {
          bg: '#0f0f0f',
          panel: '#18181b',
          elevated: '#1f1f23',
          border: '#2d2d32',
          text: '#efeff1',
          dark: '#efeff1',
          muted: '#adadb8',
          dim: '#6b6b7b',
          green: '#53fc18',
          'green-hover': '#6fff2e',
          'green-soft': 'rgb(83 252 24 / 0.15)',
          error: '#eb0400',
        },
      },
      fontFamily: {
        sans: ['var(--font-inter)', 'Inter', 'system-ui', 'sans-serif'],
      },
      animation: {
        'glow': 'glow 2s ease-in-out infinite alternate',
      },
      keyframes: {
        glow: {
          '0%': { opacity: '0.6' },
          '100%': { opacity: '1' },
        },
      },
    },
  },
  plugins: [],
}
export default config
