import type { HotRepo } from '@/lib/data';
import { Carousel } from './Carousel';
import { RepoCard } from './RepoCard';

/**
 * A horizontal "breakout repos" strip: title + subtitle over a carousel of
 * cards, each badged with the stars gained in the window. Repos we track link
 * to their detail page; newcomers we don't track link out to GitHub. Renders
 * nothing when there is no data (e.g. the scrape failed).
 */
export function HotStrip({
  title,
  subtitle,
  trendLabel,
  items,
}: {
  title: string;
  subtitle: string;
  trendLabel: string;
  items: HotRepo[];
}) {
  if (items.length === 0) return null;
  return (
    <section className="border-b border-border px-[26px] pb-5 pt-[22px]" aria-label={title}>
      <div className="mb-3 flex items-baseline gap-2.5">
        <h2 className="font-display text-lg font-extrabold">🔥 {title}</h2>
        <span className="text-xs text-muted">{subtitle}</span>
      </div>
      <Carousel ariaLabel={title}>
        {items.map((h) => (
          <RepoCard
            key={h.repo.fullName}
            repo={h.repo}
            fixedWidth
            trendStars={h.starsGained}
            trendLabel={trendLabel}
            externalHref={h.inCorpus ? undefined : h.repo.htmlUrl}
          />
        ))}
      </Carousel>
    </section>
  );
}
