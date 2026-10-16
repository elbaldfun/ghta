'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { track } from '@/lib/analytics';
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
    <section className="rounded-card border border-border bg-surface p-4">
      <h2 className="text-xs font-bold uppercase tracking-wider text-muted">{t('badgeTitle')}</h2>
      <p className="mt-1.5 text-[12.5px] leading-relaxed text-muted">{t('badgeHint')}</p>
      <img src={src} alt={t('badgeTitle')} height={20} className="mt-3 h-5 w-auto" />
      <code className="mt-3 block select-all break-all rounded-lg bg-surface2 px-2.5 py-2 font-mono text-[11px] leading-relaxed text-muted">
        {markdown}
      </code>
      <button
        type="button"
        onClick={copy}
        className="mt-2.5 flex items-center gap-1.5 rounded-full border border-border px-3 py-[5px] text-[12px] font-bold hover:border-accent hover:text-accent"
      >
        {copied ? <CheckIcon size={13} className="text-accent" /> : <CopyIcon size={13} />}
        {copied ? t('copied') : t('copyMarkdown')}
      </button>
    </section>
  );
}
