'use client';

import { useEffect, useRef, useState } from 'react';
import { Link } from '@/i18n/navigation';

const MENU_W = 220;

export interface DropdownLink {
  key: string;
  label: string;
  href: string;
  active?: boolean;
}

/**
 * A filter as a button that opens a list of links, so every choice is a
 * crawlable URL. Closes on pick, on Escape and on a click elsewhere. The
 * button is ink-outlined while a value other than "all" is chosen.
 */
export function Dropdown({
  label,
  active = false,
  items,
  align = 'left',
}: {
  label: string;
  /** A value is chosen (the button shows it). */
  active?: boolean;
  items: DropdownLink[];
  align?: 'left' | 'right';
}) {
  const [open, setOpen] = useState(false);
  // Opens to the left when a left-aligned menu would run off a narrow screen.
  const [side, setSide] = useState(align);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={(e) => {
          const r = e.currentTarget.getBoundingClientRect();
          setSide(align === 'left' && r.left + MENU_W > window.innerWidth - 8 ? 'right' : align);
          setOpen((v) => !v);
        }}
        aria-expanded={open}
        aria-haspopup="true"
        className={`flex h-11 max-w-[60vw] items-center gap-1.5 whitespace-nowrap rounded border px-3 text-sr-small transition-colors duration-150 md:h-9 md:max-w-none ${
          active ? 'border-ink text-ink' : 'border-rule text-ink-2 hover:border-ink-3 hover:text-ink'
        }`}
      >
        <span className="truncate">{label}</span>
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
      {open && (
        <ul
          className={`absolute top-full z-30 mt-1 max-h-[320px] w-[220px] max-w-[calc(100vw-32px)] overflow-y-auto rounded border border-rule bg-paper py-1 shadow-sr-pop ${
            side === 'right' ? 'right-0' : 'left-0'
          }`}
        >
          {items.map((it) => (
            <li key={it.key}>
              <Link
                href={it.href}
                onClick={() => setOpen(false)}
                aria-current={it.active ? 'true' : undefined}
                className={`block px-3 py-2 text-sr-small hover:bg-sunk ${it.active ? 'font-semibold text-ink' : 'text-ink-2'}`}
              >
                {it.label}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
