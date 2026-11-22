'use client';

import { useEffect, useRef, useState } from 'react';
import { useLocale } from 'next-intl';
import { usePathname, useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { routing, LOCALE_NAMES, LOCALE_SHORT, type Locale } from '@/i18n/routing';

/**
 * Language menu. A dropdown rather than a row of pills — the list is now long
 * enough that pills would crowd the header out.
 *
 * Options are labelled with endonyms so someone who cannot read the current
 * interface language can still find their own.
 */
export function LocaleMenu() {
  const active = useLocale() as Locale;
  const pathname = usePathname();
  const router = useRouter();
  const search = useSearchParams().toString();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  function pick(locale: Locale) {
    setOpen(false);
    // Keep the current path and query so switching language doesn't lose the
    // filters or search the visitor already applied.
    router.replace(`${pathname}${search ? `?${search}` : ''}`, { locale });
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={LOCALE_NAMES[active]}
        className="flex h-11 min-w-11 items-center justify-center gap-1 rounded border border-rule px-3 text-[13px] font-medium text-ink-2 transition-colors duration-150 hover:border-ink-3 hover:text-ink md:h-9"
      >
        {LOCALE_SHORT[active]}
        <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={3} aria-hidden>
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {open && (
        <ul
          role="listbox"
          className="absolute right-0 top-full z-50 mt-1.5 min-w-[160px] overflow-hidden rounded-lg border border-rule bg-sr-surface py-1 font-text shadow-sr-pop"
        >
          {routing.locales.map((l) => (
            <li key={l}>
              <button
                role="option"
                aria-selected={l === active}
                onClick={() => pick(l)}
                className={`flex min-h-11 w-full items-center justify-between gap-3 px-3.5 text-left text-sr-small md:min-h-9 ${
                  l === active
                    ? 'bg-sunk font-semibold text-ink'
                    : 'text-ink-2 hover:bg-sunk hover:text-ink'
                }`}
              >
                {LOCALE_NAMES[l]}
                <span className="text-sr-caption text-ink-3">{LOCALE_SHORT[l]}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
