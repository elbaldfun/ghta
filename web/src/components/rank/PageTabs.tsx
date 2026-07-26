'use client';

import { Link, usePathname } from '@/i18n/navigation';

/**
 * A section-level view switch shown at the top of related pages — e.g.
 * Leaderboard / Map, or Directory / Alternatives — so surfaces folded out of the
 * top nav stay one click apart. Highlights the tab matching the current path.
 */
export function PageTabs({ items }: { items: { href: string; label: string }[] }) {
  const pathname = usePathname();
  return (
    <div className="mb-4 -ml-[14px] flex items-center gap-1">
      {items.map((it) => {
        const active =
          it.href === '/' ? pathname === '/' : pathname === it.href || pathname.startsWith(it.href + '/');
        return (
          <Link
            key={it.href}
            href={it.href}
            aria-current={active ? 'page' : undefined}
            className={`rounded-lg border px-[13px] py-[6px] text-[12.5px] font-bold ${
              active
                ? 'border-accent bg-accent/10 text-accent'
                : 'border-transparent text-muted hover:text-fg'
            }`}
          >
            {it.label}
          </Link>
        );
      })}
    </div>
  );
}
