'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { SearchIcon } from './icons';

/**
 * Header search pill. Submits on Enter (server-backed GitHub search — a
 * per-keystroke live search would burn through the search rate limit).
 */
export function SearchBox() {
  const t = useTranslations('rank');
  const router = useRouter();
  const params = useSearchParams();
  const current = params.get('q') ?? '';
  const [value, setValue] = useState(current);

  useEffect(() => setValue(current), [current]);

  function submit() {
    const q = value.trim();
    router.push(q ? `/?q=${encodeURIComponent(q)}` : '/');
  }

  return (
    <div className="flex max-w-[460px] flex-1 items-center gap-2.5 rounded-full border border-border bg-surface2 px-4 py-[9px]">
      <SearchIcon size={15} className="shrink-0 text-muted" />
      <input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && submit()}
        placeholder={t('searchPlaceholder')}
        className="w-full bg-transparent text-[13px] text-fg outline-none placeholder:text-muted"
        aria-label={t('searchPlaceholder')}
      />
    </div>
  );
}
