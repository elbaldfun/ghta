'use client';

import { useEffect, useRef, useState } from 'react';
import { Link, usePathname } from '@/i18n/navigation';

/**
 * A nav entry that groups related pages under one label with a dropdown — e.g.
 * "AI" holding the ecosystem and topics pages. Highlights when the current page
 * is one of its items; closes on select, outside-click, and Escape.
 */
export function NavDropdown({ label, items }: { label: string; items: { href: string; label: string }[] }) {
  const pathname = usePathname();
  const active = items.some((it) => pathname === it.href || pathname.startsWith(it.href + '/'));
  const [open, setOpen] = useState(false);
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
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className={`flex items-center gap-1 rounded-lg px-[13px] py-[7px] text-[12.5px] font-bold ${
          active ? 'bg-accent text-accent-fg' : 'text-muted hover:text-fg'
        }`}
      >
        {label}
        <span className="text-[9px]">▾</span>
      </button>
      {open && (
        <div className="absolute left-0 top-full z-30 mt-1 w-[160px] rounded-lg border border-border bg-surface p-1 shadow-card-hover">
          {items.map((it) => {
            const itemActive = pathname === it.href || pathname.startsWith(it.href + '/');
            return (
              <Link
                key={it.href}
                href={it.href}
                onClick={() => setOpen(false)}
                className={`block rounded-md px-2.5 py-1.5 text-[12.5px] font-semibold hover:bg-surface2 ${
                  itemActive ? 'text-accent' : 'text-fg'
                }`}
              >
                {it.label}
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
