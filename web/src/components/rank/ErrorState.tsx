'use client';

import { useState, useTransition } from 'react';
import { useTranslations } from 'next-intl';
import { useRouter } from '@/i18n/navigation';

/**
 * Friendly data-load failure card with a retry button (server-component pages
 * surface fetch errors as values, so this re-runs them via router.refresh).
 * Internal error details (HTTP status etc.) stay out of the user's face.
 */
export function ErrorState() {
  const t = useTranslations('rank');
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [attempted, setAttempted] = useState(false);

  function retry() {
    setAttempted(true);
    startTransition(() => router.refresh());
  }

  return (
    <div className="flex flex-col items-center gap-3 rounded-card border border-border bg-surface px-6 py-12 text-center">
      <span aria-hidden="true" className="text-2xl">
        📡
      </span>
      <div>
        <p className="text-[14px] font-bold">{t('loadError')}</p>
        <p className="mt-1 text-[12.5px] text-muted">
          {attempted && !pending ? t('loadErrorPersists') : t('loadErrorHint')}
        </p>
      </div>
      <button
        onClick={retry}
        disabled={pending}
        className="rounded-full border border-accent px-4 py-1.5 text-[12.5px] font-bold text-accent transition-colors hover:bg-accent/10 disabled:opacity-50"
      >
        {pending ? t('retrying') : t('retry')}
      </button>
    </div>
  );
}
