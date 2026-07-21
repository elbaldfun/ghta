'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { applyConsent, readConsent, writeConsent, type ConsentChoice } from '@/lib/consent';

/**
 * Cookie banner gating Consent Mode. Only rendered when GA is configured —
 * with no analytics script there is nothing to consent to.
 *
 * Mounts hidden and reveals itself after reading localStorage, so the server
 * HTML never shows a banner the visitor already dismissed.
 */
export function ConsentBanner() {
  const t = useTranslations('consent');
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (!process.env.NEXT_PUBLIC_GA_ID) return;
    setVisible(readConsent() === null);
  }, []);

  function choose(choice: ConsentChoice) {
    writeConsent(choice);
    applyConsent(choice);
    setVisible(false);
  }

  if (!visible) return null;

  return (
    <div
      role="dialog"
      aria-label={t('title')}
      className="fixed inset-x-0 bottom-0 z-50 border-t border-border bg-surface/95 backdrop-blur"
    >
      <div className="mx-auto flex max-w-screen-xl flex-col gap-3 px-7 py-4 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-xs leading-relaxed text-muted">
          {t('message')}{' '}
          <Link href="/privacy" className="font-semibold text-accent hover:underline">
            {t('learnMore')}
          </Link>
        </p>
        <div className="flex shrink-0 gap-2">
          <button
            onClick={() => choose('denied')}
            className="rounded-lg border border-border px-3.5 py-1.5 text-xs font-semibold text-muted hover:text-fg"
          >
            {t('reject')}
          </button>
          <button
            onClick={() => choose('granted')}
            className="rounded-lg bg-accent px-3.5 py-1.5 text-xs font-bold text-accent-fg"
          >
            {t('accept')}
          </button>
        </div>
      </div>
    </div>
  );
}
