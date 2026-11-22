'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { track } from '@/lib/analytics';
import { CopyIcon, CheckIcon } from './icons';

/**
 * The install-command line with a click-to-copy affordance. Copying an install
 * command is the strongest "this repo was useful" intent signal a detail page
 * has, so it emits a `copy_install` conversion event.
 */
export function CopyCommand({ command, repo }: { command: string; repo: string }) {
  const t = useTranslations('rank');
  const [copied, setCopied] = useState(false);

  async function copy() {
    // Record intent first: the click IS the conversion signal, even if the
    // clipboard API is blocked (insecure context / denied permission).
    track('copy_install', { repo, command });
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable — the event is already logged */
    }
  }

  return (
    <button
      type="button"
      onClick={copy}
      aria-label={t('copyCommand')}
      className="group flex min-h-11 min-w-0 items-center gap-2 text-left md:min-h-0"
    >
      <code className="truncate font-code text-[13px] text-ink-2 group-hover:text-ink">{command}</code>
      {copied ? (
        <CheckIcon size={13} className="shrink-0 text-ink" />
      ) : (
        <CopyIcon size={13} className="shrink-0 text-ink-3 group-hover:text-ink" />
      )}
    </button>
  );
}
