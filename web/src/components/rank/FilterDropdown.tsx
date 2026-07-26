'use client';

import { useEffect, useRef, useState } from 'react';
import { Link } from '@/i18n/navigation';

export interface DropdownItem {
  key: string;
  label: string;
  href: string;
  active: boolean;
}

/**
 * A dropdown of navigation links that closes when you pick one (a native
 * <details> stays open across client-side navigation) and when you click away.
 */
export function FilterDropdown({
  label,
  active,
  items,
}: {
  label: string;
  active: boolean;
  items: DropdownItem[];
}) {
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
        className="flex items-center gap-1 rounded-md border border-border px-2.5 py-1 text-[11.5px] font-semibold text-muted hover:text-fg"
      >
        <span className={active ? 'text-accent' : ''}>{label}</span>
        <span className="text-[9px]">▾</span>
      </button>
      {open && (
        <div className="absolute left-0 top-full z-20 mt-1 max-h-[320px] w-[200px] overflow-y-auto rounded-lg border border-border bg-surface p-1 shadow-card-hover">
          {items.map((it) => (
            <Link
              key={it.key}
              href={it.href}
              onClick={() => setOpen(false)}
              className={`block rounded-md px-2.5 py-1.5 text-[12px] font-semibold hover:bg-surface2 ${
                it.active ? 'text-accent' : 'text-fg'
              }`}
            >
              {it.label}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
