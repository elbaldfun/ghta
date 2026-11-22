'use client';

import { Link, usePathname } from '@/i18n/navigation';

/**
 * A header nav item. usePathname returns the locale-stripped path (e.g.
 * "/apps"), so the active state survives client-side navigation. `alsoOn`
 * lists other paths the item stands for. Active is ink with a 2px ink
 * underline; the rest are ink-2. 44px tall for touch.
 */
export function NavLink({ href, label, alsoOn = [] }: { href: string; label: string; alsoOn?: string[] }) {
  const pathname = usePathname();
  const on = (h: string) => (h === '/' ? pathname === '/' : pathname === h || pathname.startsWith(h + '/'));
  const active = on(href) || alsoOn.some(on);

  return (
    <Link
      href={href}
      aria-current={active ? 'page' : undefined}
      className={`flex h-11 shrink-0 items-center whitespace-nowrap border-b-2 transition-colors duration-150 ${
        active ? 'border-ink text-ink' : 'border-transparent text-ink-2 hover:text-ink'
      }`}
    >
      {label}
    </Link>
  );
}
