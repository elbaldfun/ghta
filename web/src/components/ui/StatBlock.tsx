import type { ReactNode } from 'react';

const SIZE = {
  lg: 'text-[36px] md:text-[56px]',
  md: 'text-[22px] md:text-[28px]',
  sm: 'text-[20px]',
} as const;

/** A labelled figure: "今日 / ▲1,749". Numbers are tabular so blocks align. */
export function StatBlock({
  label,
  value,
  sub,
  tone = 'ink',
  size = 'md',
  className = '',
}: {
  label: ReactNode;
  value: ReactNode;
  sub?: ReactNode;
  tone?: 'ink' | 'signal';
  size?: keyof typeof SIZE;
  className?: string;
}) {
  return (
    <div className={`flex flex-col gap-0.5 ${className}`}>
      <span className="text-sr-caption text-ink-3">{label}</span>
      <span
        className={`${SIZE[size]} font-semibold leading-tight tracking-[-0.02em] tabular-nums ${tone === 'signal' ? 'text-signal' : 'text-ink'}`}
      >
        {value}
      </span>
      {sub ? <span className="text-sr-caption text-ink-3">{sub}</span> : null}
    </div>
  );
}
