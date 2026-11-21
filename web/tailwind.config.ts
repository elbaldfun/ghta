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

        // Design tokens v1 ("数据报刊", src/styles/tokens.css). New UI uses
        // only these; the legacy names above go away page by page.
        paper: 'rgb(var(--sr-paper) / <alpha-value>)',
        'sr-surface': 'rgb(var(--sr-surface) / <alpha-value>)',
        sunk: 'rgb(var(--sr-sunk) / <alpha-value>)',
        rule: 'rgb(var(--sr-rule) / <alpha-value>)',
        bar: 'rgb(var(--sr-bar) / <alpha-value>)',
        ink: {
          DEFAULT: 'rgb(var(--sr-ink) / <alpha-value>)',
          2: 'rgb(var(--sr-ink-2) / <alpha-value>)',
          3: 'rgb(var(--sr-ink-3) / <alpha-value>)',
        },
        signal: {
          DEFAULT: 'rgb(var(--sr-signal) / <alpha-value>)',
          bar: 'rgb(var(--sr-signal-bar) / <alpha-value>)',
          soft: 'rgb(var(--sr-signal-soft) / <alpha-value>)',
          ink: 'rgb(var(--sr-signal-ink) / <alpha-value>)',
        },
        brand: 'rgb(var(--sr-brand) / <alpha-value>)',
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
        // Design tokens v1: news = headlines (Newsreader + CJK serif),
        // text = UI, tables and numbers (Inter + CJK sans), code = snippets.
        news: ['var(--sr-font-news)'],
        text: ['var(--sr-font-text)'],
        code: ['var(--sr-font-code)'],
      },
      // Design tokens v1 type scale, 7 steps. Pair the large steps with a
      // smaller one below md (e.g. `text-sr-h1 md:text-sr-display`).
      fontSize: {
        'sr-display': ['72px', { lineHeight: '1.05', letterSpacing: '-0.02em' }],
        'sr-h1': ['44px', { lineHeight: '1.1', letterSpacing: '-0.015em' }],
        'sr-h2': ['28px', { lineHeight: '1.25', letterSpacing: '-0.01em' }],
        'sr-lead': ['20px', { lineHeight: '1.5' }],
        'sr-body': ['16px', { lineHeight: '1.6' }],
        'sr-small': ['14px', { lineHeight: '1.5' }],
        'sr-caption': ['12px', { lineHeight: '1.4', letterSpacing: '0.04em' }],
      },
      boxShadow: {
        'sr-pop': 'var(--sr-shadow-pop)',
        'card-hover': '0 8px 22px rgba(20,20,40,0.12)',
        arrow: '0 4px 14px rgba(20,20,40,0.16)',
      },
    },
  },
  plugins: [],
};

export default config;
