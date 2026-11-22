'use client';

import { useEffect, useState } from 'react';
import { MoonIcon, SunIcon } from './icons';

type Theme = 'light' | 'dark';

/** Theme toggle (sun in dark mode, moon in light): a 4px-cornered ruled button. */
export function ThemePill() {
  const [theme, setTheme] = useState<Theme | null>(null);

  useEffect(() => {
    const stored = (localStorage.getItem('theme') as Theme | null) ?? null;
    const initial =
      stored ?? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    setTheme(initial);
  }, []);

  function toggle() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
  }

  return (
    <button
      onClick={toggle}
      aria-label="Toggle color theme"
      className="flex h-11 w-11 items-center justify-center rounded border border-rule text-ink-2 transition-colors duration-150 hover:border-ink-3 hover:text-ink md:h-9"
    >
      {theme === 'dark' ? <SunIcon size={14} /> : <MoonIcon size={14} />}
    </button>
  );
}
