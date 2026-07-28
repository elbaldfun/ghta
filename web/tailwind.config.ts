import type { Config } from 'tailwindcss';

// Colors are driven by CSS variables (see globals.css) so light/dark themes and
// the source/category palette stay in one place.
const config: Config = {
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: 'rgb(var(--bg) / <alpha-value>)',
        surface: 'rgb(var(--surface) / <alpha-value>)',
        surface2: 'rgb(var(--surface2) / <alpha-value>)',
        border: 'rgb(var(--border) / <alpha-value>)',
        fg: 'rgb(var(--fg) / <alpha-value>)',
        muted: 'rgb(var(--muted) / <alpha-value>)',
        accent: 'rgb(var(--accent) / <alpha-value>)',
        accent2: 'rgb(var(--accent2) / <alpha-value>)',
        heat0: 'rgb(var(--heat0) / <alpha-value>)',
        heat1: 'rgb(var(--heat1) / <alpha-value>)',
        heat2: 'rgb(var(--heat2) / <alpha-value>)',
        heat3: 'rgb(var(--heat3) / <alpha-value>)',
        heat4: 'rgb(var(--heat4) / <alpha-value>)',
        'accent-fg': 'rgb(var(--accent-fg) / <alpha-value>)',
        up: 'rgb(var(--up) / <alpha-value>)',
        down: 'rgb(var(--down) / <alpha-value>)',
      },
      borderRadius: {
        card: 'var(--radius-card)',
      },
      spacing: {
        // The page's horizontal gutter on desktop. ONLY PageShell (and gutter-
        // aligned edge cases inside it) may reference this; pages never hand-
        // write horizontal padding — see scripts/layout-guard.mjs.
        gutter: '28px',
      },
      fontFamily: {
        sans: ['var(--font-sans)', 'system-ui', 'sans-serif'],
        display: ['var(--font-display)', 'system-ui', 'sans-serif'],
        mono: ['var(--font-mono)', 'ui-monospace', 'monospace'],
      },
      boxShadow: {
        'card-hover': '0 8px 22px rgba(20,20,40,0.12)',
        arrow: '0 4px 14px rgba(20,20,40,0.16)',
      },
    },
  },
  plugins: [],
};

export default config;
