'use client';

import { Link, usePathname } from '@/i18n/navigation';

/**
 * A nav item that highlights when it points at the current page. usePathname
 * returns the locale-stripped path (e.g. "/apps"), so active state survives
 * client-side navigation — the header stops showing "排行榜" as selected on every
 * page.
 */
export function NavLink({ href, label }: { href: string; label: string }) {
  const pathname = usePathname();
  const active = href === '/' ? pathname === '/' : pathname === href || pathname.startsWith(href + '/');

  return (
    <Link
      href={href}
      aria-current={active ? 'page' : undefined}
      className={`rounded-lg px-[13px] py-[7px] text-[12.5px] font-bold ${
        active ? 'bg-accent text-accent-fg' : 'text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );
}
