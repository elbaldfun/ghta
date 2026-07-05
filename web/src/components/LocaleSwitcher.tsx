'use client';

import { useLocale } from 'next-intl';
import { usePathname, useRouter } from '@/i18n/navigation';
import { routing } from '@/i18n/routing';

export function LocaleSwitcher() {
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();

  return (
    <div className="flex items-center gap-1 text-sm">
      {routing.locales.map((l) => (
        <button
          key={l}
          onClick={() => router.replace(pathname, { locale: l })}
          aria-current={l === locale ? 'true' : undefined}
          className={l === locale ? 'font-semibold text-fg' : 'text-muted hover:text-fg'}
        >
          {l.toUpperCase()}
        </button>
      ))}
    </div>
  );
}
