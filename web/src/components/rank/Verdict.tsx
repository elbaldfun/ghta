import { getLocale, getTranslations } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import type { AppItem, OS, Verdict, VerdictEvidence, VerdictSkip } from '@/lib/data';
import { fmtTotal, srButton } from '@/components/ui';
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
 * The page's pick and its evidence, worded from the fields the boards job
 * chose it by (internal/boards/verdict.go). Nothing here decides anything.
 * Without a verdict the block stays and says why there is none yet, so the
 * page's shape does not depend on whether tonight's run has finished.
 *
 * `subject` names the page in the heading ("Local LLM"); `apps` (shelves)
 * supplies the pick's per-platform download links.
 */
export async function PickBlock({
  verdict,
  scope,
  subject,
  apps = [],
  pool,
  freshDays,
}: {
  verdict: Verdict | null;
  scope: Scope;
  subject: string;
  apps?: AppItem[];
  pool?: number;
  freshDays?: number;
}) {
  const t = await getTranslations('verdict');
  const p = await getTranslations('pick');
  const locale = await getLocale();
  const num = new Intl.NumberFormat(locale);
  const list = new Intl.ListFormat(locale, { type: 'conjunction' });

  if (!verdict) {
    return (
      <section aria-labelledby="pick" className="flex flex-col gap-2 border-y-2 border-ink py-5 md:py-6">
        <p className="text-sr-caption font-semibold tracking-[0.06em] text-ink-3">{t('label')}</p>
        <h2 id="pick" className="font-news text-[24px] font-medium leading-tight md:text-[30px]">
          {p('emptyHead')}
        </h2>
        <p className="max-w-[720px] text-sr-small text-ink-2 md:text-sr-body">
          {scope === 'shelf'
            ? p('emptyShelf', { fresh: freshDays ?? 90 })
            : p('emptyDomain', { pool: pool ?? 10, fresh: freshDays ?? 90 })}
        </p>
      </section>
    );
  }

  const { pick, riser, skipped } = verdict;
  const name = displayName(scope, pick);

  const facts: string[] = [
    verdict.basis === 'stars'
      ? t('posStars', { pos: pick.position, stars: fmtTotal(pick.stars) })
      : t('posBest', { pos: pick.position, stars: fmtTotal(pick.stars) }),
  ];
  if (pick.d30 != null && pick.d30 > 0) facts.push(t('gain', { n: num.format(pick.d30) }));
  if (pick.pushedDaysAgo >= 0) facts.push(t('pushed', { days: pick.pushedDaysAgo }));
  facts.push(pick.releaseDaysAgo >= 0 ? t('release', { days: pick.releaseDaysAgo }) : t('noRelease'));
  if (pick.downloads && pick.downloads.length > 0) {
    facts.push(t('downloads', { platforms: list.format(pick.downloads.map((os) => OS_LABEL[os] ?? os)) }));
  }

  const skipReason = (s: VerdictSkip) => {
    const n = displayName(scope, s);
    if (s.reason === 'reading') return t('skipReading', { name: n });
    if (s.reason === 'no-build') return t('skipNoBuild', { name: n });
    return s.pushedDaysAgo < 0 ? t('skipUnknown', { name: n }) : t('skipStale', { name: n, days: s.pushedDaysAgo });
  };

  // The side list: why the projects above the pick are not it, who rose
  // fastest, and the pick's move in its ranking.
  const why: { k: string; v: string; href?: string }[] = skipped.map((s) => ({
    k: p('whyNot', { pos: s.position }),
    v: skipReason(s),
  }));
  if (riser && riser.d30 != null && riser.externalId !== pick.externalId) {
    why.push({
      k: p('riser'),
      v: p('riserVal', { name: displayName(scope, riser), n: num.format(riser.d30), pos: riser.position }),
      href: `/repo/${riser.externalId}`,
    });
  }
  if (pick.rankNow && pick.rankPrev && pick.rankNow !== pick.rankPrev) {
    why.push({
      k: p('rank'),
      v: t(pick.rankNow < pick.rankPrev ? 'rankUp' : 'rankDown', { now: pick.rankNow, prev: pick.rankPrev }),
    });
  }

  // Shelves lead with downloads for the platforms the pick ships; domains
  // with the profile and the repository.
  const app = scope === 'shelf' ? apps.find((a) => a.externalId === pick.externalId) : undefined;
  const builds = (app?.downloads ?? []).filter((d) => /^https?:\/\//.test(d.url)).slice(0, 2);

  return (
    <section
      aria-labelledby="pick"
      className="grid gap-5 border-y-2 border-ink py-5 md:py-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)] lg:gap-12"
    >
      <div className="flex min-w-0 flex-col gap-3">
        <p className="text-sr-caption font-semibold tracking-[0.06em] text-signal">{t('label')}</p>
        <h2 id="pick" className="break-words font-news text-[26px] font-medium leading-[1.15] [overflow-wrap:anywhere] md:text-[34px]">
          {scope === 'shelf' ? p('headShelf', { subject, name }) : p('headDomain', { name })}
        </h2>
        <p className="max-w-[720px] text-sr-body text-ink-2">{facts.join(t('sep'))}</p>
        <div className="flex flex-wrap gap-2 pt-1">
          {builds.length > 0 ? (
            builds.map((d, i) => (
              <a
                key={d.os}
                href={d.url}
                rel="nofollow noopener"
                className={srButton(i === 0 ? 'solid' : 'outline')}
              >
                {i === 0 ? p('downloadOs', { os: OS_LABEL[d.os] }) : p('osOnly', { os: OS_LABEL[d.os] })}
              </a>
            ))
          ) : (
            <Link href={`/repo/${pick.externalId}`} className={srButton('solid')}>
              {p('profile')}
            </Link>
          )}
          <a href={`https://github.com/${pick.externalId}`} target="_blank" rel="noopener noreferrer" className={srButton('outline')}>
            GitHub ↗
          </a>
        </div>
      </div>
      {why.length > 0 && (
        <dl className="flex min-w-0 flex-col self-start border-t border-rule lg:border-t-0">
          {why.map((x, i) => (
            <div key={i} className="flex flex-col gap-0.5 border-b border-rule py-2.5">
              <dt className="text-sr-caption text-ink-3">{x.k}</dt>
              <dd className="break-words text-sr-small text-ink [overflow-wrap:anywhere]">
                {x.href ? (
                  <Link href={x.href} className="hover:underline">
                    {x.v}
                  </Link>
                ) : (
                  x.v
                )}
              </dd>
            </div>
          ))}
        </dl>
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
    <p className="max-w-[900px] text-[13px] leading-relaxed text-ink-2">
      {scope === 'shelf'
        ? t('ruleShelf', { count: count ?? 0, fresh, date })
        : t('ruleDomain', { pool: pool ?? 10, fresh, date })}
    </p>
  );
}

export type { VerdictEvidence };
