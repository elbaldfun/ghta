'use client';

import { useEffect, useState } from 'react';

type Theme = 'light' | 'dark';

export function ThemeToggle() {
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
      className="rounded-md border border-border px-2 py-1 text-sm text-muted hover:text-fg"
    >
      {theme === 'dark' ? '☀' : '☾'}
    </button>
  );
}
