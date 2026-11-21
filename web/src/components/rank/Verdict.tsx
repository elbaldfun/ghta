import type { ReactNode } from 'react';
import { getLocale, getTranslations } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import type { OS, Verdict, VerdictEvidence, VerdictSkip } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';
import { appName } from './AppCard';

const OS_LABEL: Record<OS, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
  web: 'Web',
};

type Scope = Verdict['scope'];

/** A shelf shows the app's name as its card does; a domain board the repo's. */
function displayName(scope: Scope, e: { externalId: string; name: string }): string {
  return scope === 'shelf' ? appName(e.name) : e.name;
}

/**
 * The page's pick and its evidence, one sentence each, worded from the
 * fields the boards job chose it by (internal/boards/verdict.go). Nothing
 * here decides anything: no verdict, no block.
 */
export async function VerdictBlock({ verdict }: { verdict: Verdict | null }) {
  if (!verdict) return null;
  const t = await getTranslations('verdict');
  const locale = await getLocale();
  const num = new Intl.NumberFormat(locale);
  const list = new Intl.ListFormat(locale, { type: 'conjunction' });
  const { scope, pick, riser, skipped } = verdict;

  const strong = (e: VerdictEvidence) => {
    const Tag = (chunks: ReactNode) => (
      <Link href={`/repo/${e.externalId}`} title={e.externalId} className="font-extrabold text-fg hover:text-accent">
        {chunks}
      </Link>
    );
    return Tag;
  };

  const facts: string[] = [
    verdict.basis === 'stars'
      ? t('posStars', { pos: pick.position, stars: formatCompact(pick.stars) })
      : t('posBest', { pos: pick.position, stars: formatCompact(pick.stars) }),
  ];
  if (pick.d30 != null && pick.d30 > 0) facts.push(t('gain', { n: num.format(pick.d30) }));
  facts.push(t('pushed', { days: pick.pushedDaysAgo }));
  facts.push(pick.releaseDaysAgo >= 0 ? t('release', { days: pick.releaseDaysAgo }) : t('noRelease'));
  if (pick.downloads && pick.downloads.length > 0) {
    facts.push(t('downloads', { platforms: list.format(pick.downloads.map((os) => OS_LABEL[os] ?? os)) }));
  }
  if (pick.rankNow && pick.rankPrev && pick.rankNow !== pick.rankPrev) {
    facts.push(t(pick.rankNow < pick.rankPrev ? 'rankUp' : 'rankDown', { now: pick.rankNow, prev: pick.rankPrev }));
  }

  const skip = (s: VerdictSkip) => {
    const name = displayName(scope, s);
    if (s.reason === 'reading') return t('skipReading', { name });
    if (s.reason === 'no-build') return t('skipNoBuild', { name });
    return s.pushedDaysAgo < 0 ? t('skipUnknown', { name }) : t('skipStale', { name, days: s.pushedDaysAgo });
  };

  return (
    <section className="mb-3 max-w-[85ch] rounded-card border border-border bg-surface px-4 py-3">
      <div className="mb-1 text-[11px] font-bold uppercase tracking-wide text-accent">{t('label')}</div>
      <p className="text-[13.5px] leading-relaxed text-fg">
        {t.rich('pickLine', { name: displayName(scope, pick), facts: facts.join(t('sep')), b: strong(pick) })}
      </p>
      {skipped.length > 0 && (
        <p className="mt-1 text-[12px] leading-relaxed text-muted">
          {t('skipLine', { items: skipped.map(skip).join(t('sep')) })}
        </p>
      )}
      {riser && riser.d30 != null && (
        <p className="mt-1 text-[12px] leading-relaxed text-muted">
          {t.rich('riserLine', {
            name: displayName(scope, riser),
            n: num.format(riser.d30),
            pos: riser.position,
            b: strong(riser),
          })}
        </p>
      )}
    </section>
  );
}

/** The page's ranking rule in one line, with when the data was computed. */
export async function RankingRule({
  scope,
  computedAt,
  pool,
  freshDays,
  count,
}: {
  scope: Scope;
  computedAt: string | null;
  pool?: number;
  freshDays?: number;
  count?: number;
}) {
  const t = await getTranslations('verdict');
  const locale = await getLocale();
  const at = computedAt ? new Date(computedAt) : null;
  const date =
    at && !Number.isNaN(at.getTime())
      ? new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeZone: 'UTC' }).format(at)
      : '—';
  const fresh = freshDays ?? 90;
  return (
    <p className="mb-4 max-w-[85ch] text-[11.5px] leading-relaxed text-muted">
      {scope === 'shelf'
        ? t('ruleShelf', { count: count ?? 0, fresh, date })
        : t('ruleDomain', { pool: pool ?? 10, fresh, date })}
    </p>
  );
}
