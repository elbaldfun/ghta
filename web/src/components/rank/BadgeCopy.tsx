'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { track } from '@/lib/analytics';
import { srButton } from '@/components/ui/controls';
import { CheckIcon, CopyIcon } from './icons';

/**
 * The README badge for a repository: a preview and the Markdown that embeds
 * it, linking back to the detail page. A maintainer pasting it is the one
 * thing a visitor can take away from the page, so the copy is tracked.
 */
export function BadgeCopy({ src, href, repo }: { src: string; href: string; repo: string }) {
  const t = useTranslations('rank');
  const [copied, setCopied] = useState(false);
  const markdown = `[![StarRank](${src})](${href})`;

  async function copy() {
    track('copy_badge', { repo });
    try {
      await navigator.clipboard.writeText(markdown);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable — the Markdown stays selectable below */
    }
  }

  return (
    <section className="flex flex-col gap-3 border-t-2 border-ink pt-3">
      <h2 className="text-[18px] font-semibold">{t('badgeTitle')}</h2>
      <p className="text-sr-small text-ink-2">{t('badgeHint')}</p>
      <img src={src} alt={t('badgeTitle')} height={20} className="h-5 w-auto self-start" />
      <div className="flex flex-col gap-2 rounded border border-rule bg-sunk p-3">
        <code className="block select-all break-all font-code text-[12px] leading-relaxed text-ink-2">{markdown}</code>
        <button type="button" onClick={copy} className={srButton('outline', 'self-start')}>
          {copied ? <CheckIcon size={13} /> : <CopyIcon size={13} />}
          {copied ? t('copied') : t('copyMarkdown')}
        </button>
      </div>
    </section>
  );
}
