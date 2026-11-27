'use client';

import { useEffect, useRef } from 'react';
import { Link } from '@/i18n/navigation';

export interface GenreChip {
  key: string;
  label: string;
  /** Apps of this genre on the overall chart; null for "all", 0 for none. */
  count: number | null;
  href: string;
  active: boolean;
}

/**
 * The chart's categories laid out flat, busiest first, each a plain link so
 * every genre chart stays crawlable. One scrolling row on a phone, so the
 * chart keeps the first screen; wrapped from lg up.
 */
export function GenreChips({ items, label }: { items: GenreChip[]; label: string }) {
  const row = useRef<HTMLElement>(null);

  // On a phone the active chip may sit off to the right: bring it into view.
  useEffect(() => {
    const el = row.current?.querySelector<HTMLElement>('[aria-current]');
    if (el && row.current && row.current.scrollWidth > row.current.clientWidth) {
      row.current.scrollLeft = el.offsetLeft - row.current.clientWidth / 2 + el.clientWidth / 2;
    }
  }, []);

  return (
    <nav
      ref={row}
      aria-label={label}
      className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 lg:mx-0 lg:flex-wrap lg:overflow-visible lg:px-0 lg:pb-0"
    >
      {items.map((it) => (
        <Link
          key={it.key}
          href={it.href}
          aria-current={it.active ? 'true' : undefined}
          className={`flex h-9 shrink-0 items-center gap-1.5 whitespace-nowrap rounded border px-3 text-[13px] font-medium transition-colors duration-150 md:h-8 ${
            it.active
              ? 'border-ink bg-ink text-paper'
              : it.count === 0
                ? 'border-rule text-ink-3 hover:border-ink-3 hover:text-ink'
                : 'border-rule text-ink-2 hover:border-ink-3 hover:text-ink'
          }`}
        >
          {it.label}
          {it.count ? (
            <span className={`tabular-nums ${it.active ? 'text-paper/70' : 'text-ink-3'}`}>{it.count}</span>
          ) : null}
        </Link>
      ))}
    </nav>
  );
}
