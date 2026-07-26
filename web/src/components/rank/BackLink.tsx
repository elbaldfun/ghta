'use client';

import { useRouter } from '@/i18n/navigation';
import { useTranslations } from 'next-intl';
import { BackIcon } from './icons';

/**
 * Back control for detail pages. Returns the visitor to wherever they came from
 * — the breakout board, a ranking, search results, with scroll position intact —
 * by walking browser history. When there is no in-app history to walk (a deep
 * link opened as the tab's first page, a shared URL, a search-engine landing),
 * it falls back to a sensible destination instead of leaving the site.
 */
export function BackLink({ fallback = '/' }: { fallback?: string }) {
  const t = useTranslations('rank');
  const router = useRouter();

  function onBack() {
    if (typeof window !== 'undefined' && window.history.length > 1) {
      router.back();
    } else {
      router.push(fallback);
    }
  }

  return (
    <button
      type="button"
      onClick={onBack}
      className="mb-4 flex w-fit items-center gap-1.5 text-xs font-semibold text-muted hover:text-fg"
    >
      <BackIcon size={14} />
      {t('back')}
    </button>
  );
}
