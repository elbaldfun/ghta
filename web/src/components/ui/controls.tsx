import type { ReactNode } from 'react';
import { Link } from '@/i18n/navigation';

type ButtonVariant = 'solid' | 'outline' | 'quiet';

/**
 * Class string for a button or a link styled as one. Square-ish corners (4px)
 * and ink, never the signal colour: buttons are actions, not momentum.
 * Height is 44px on touch widths and 36px from md up.
 */
export function srButton(variant: ButtonVariant = 'outline', className = ''): string {
  const base =
    'inline-flex h-11 items-center justify-center gap-1.5 whitespace-nowrap rounded px-3.5 text-sr-small font-semibold transition-colors duration-150 md:h-9';
  const look = {
    solid: 'bg-ink text-paper hover:bg-ink-2',
    outline: 'border border-ink text-ink hover:bg-sunk',
    quiet: 'border border-rule text-ink-2 hover:border-ink-3 hover:text-ink',
  }[variant];
  return `${base} ${look} ${className}`;
}

/** An outlined label: language, domain, topic. `signal` marks "new". */
export function Tag({
  children,
  tone = 'plain',
  className = '',
}: {
  children: ReactNode;
  tone?: 'plain' | 'signal';
  className?: string;
}) {
  const look = tone === 'signal' ? 'border-signal text-signal' : 'border-rule text-ink-2';
  return (
    <span className={`inline-flex items-center rounded border px-2 py-0.5 text-[13px] leading-snug ${look} ${className}`}>
      {children}
    </span>
  );
}

export interface SegmentItem {
  key: string;
  label: ReactNode;
  href: string;
  active?: boolean;
}

/**
 * A row of mutually exclusive views (sort order, platform, chart type) as
 * links, so every state is a crawlable URL. The active segment is filled ink.
 */
export function Segmented({
  items,
  label,
  className = '',
}: {
  items: SegmentItem[];
  /** Names the group for screen readers, e.g. "Sort". */
  label: string;
  className?: string;
}) {
  return (
    <nav aria-label={label} className={`inline-flex shrink-0 self-start overflow-hidden rounded border border-ink ${className}`}>
      {items.map((it) => (
        <Link
          key={it.key}
          href={it.href}
          aria-current={it.active ? 'true' : undefined}
          className={`flex h-11 items-center whitespace-nowrap px-3 text-[13px] font-medium transition-colors duration-150 md:h-[34px] ${
            it.active ? 'bg-ink text-paper' : 'text-ink-2 hover:bg-sunk hover:text-ink'
          }`}
        >
          {it.label}
        </Link>
      ))}
    </nav>
  );
}
