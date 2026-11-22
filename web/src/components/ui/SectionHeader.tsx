import type { ElementType, ReactNode } from 'react';

/**
 * Opens a section the broadsheet way: a 2px ink rule, the title, and an
 * optional aside on the right (a "See all →" link, a unit note).
 */
export function SectionHeader({
  title,
  as: Tag = 'h2',
  id,
  aside,
  className = '',
}: {
  title: ReactNode;
  as?: ElementType;
  id?: string;
  aside?: ReactNode;
  className?: string;
}) {
  return (
    <div className={`flex items-baseline justify-between gap-4 border-t-2 border-ink py-3 ${className}`}>
      <Tag id={id} className="font-text text-[22px] font-semibold leading-tight tracking-[-0.01em] text-ink">
        {title}
      </Tag>
      {aside ? <div className="text-sr-small text-ink-2">{aside}</div> : null}
    </div>
  );
}

/**
 * The small label above a headline: what rule picked this story. Signal tone
 * for momentum stories, muted for the rest.
 */
export function Kicker({
  children,
  tone = 'signal',
  className = '',
}: {
  children: ReactNode;
  tone?: 'signal' | 'muted';
  className?: string;
}) {
  return (
    <p
      className={`text-sr-caption font-semibold tracking-[0.1em] ${tone === 'signal' ? 'text-signal' : 'text-ink-3'} ${className}`}
    >
      {children}
    </p>
  );
}
