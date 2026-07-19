'use client';

import { useLocale } from 'next-intl';
import { usePathname, useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { routing } from '@/i18n/routing';

const LABELS: Record<string, string> = { zh: '中', en: 'EN' };

export function LocalePills() {
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const search = useSearchParams().toString();

  return (
    <div className="flex items-center gap-1.5">
      {routing.locales.map((l) => {
        const active = l === locale;
        return (
          <button
            key={l}
            onClick={() => router.replace(`${pathname}${search ? `?${search}` : ''}`, { locale: l })}
            aria-current={active ? 'true' : undefined}
            className={`rounded-full border px-3 py-1.5 text-[11px] font-semibold ${
              active
                ? 'border-accent bg-accent text-accent-fg'
                : 'border-border bg-transparent text-muted hover:text-fg'
            }`}
          >
            {LABELS[l] ?? l.toUpperCase()}
          </button>
        );
      })}
    </div>
  );
}
