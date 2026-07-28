import type { ElementType, ReactNode } from 'react';

/**
 * THE single source of page width and horizontal gutter. Every aligned surface
 * — header inner row, footer inner row, page bodies, full-bleed sections'
 * inner content — wraps in this, so left/right edges line up by construction
 * instead of by hand-tuned paddings (which drifted: 26px vs 28px vs 16+10px).
 *
 * `gutter={false}` is for containers whose columns manage their own inset
 * (e.g. the home grid, where the sidebar owns its background up to the
 * container edge and the content column applies the gutter itself).
 *
 * scripts/layout-guard.mjs fails the build if `max-w-screen-xl`, `px-[26px]`
 * or `px-7` appear anywhere else.
 */
export function PageShell({
  as,
  gutter = true,
  className,
  children,
}: {
  as?: ElementType;
  gutter?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const Tag = (as ?? 'div') as ElementType;
  return (
    <Tag
      className={`mx-auto w-full max-w-screen-xl ${gutter ? 'px-4 lg:px-gutter' : ''} ${className ?? ''}`}
    >
      {children}
    </Tag>
  );
}
