import { useLocale, useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { appPath, countryName, formatPrice, iconAt, type NewEntry } from '@/lib/api/ios';
import { fmtInt } from '@/components/ui';

/**
 * One app in the new-apps view: its place in this view, where that place
 * comes from ("United States · Top Paid · Productivity #1"), and when it
 * first came out.
 */
export function NewRow({ entry }: { entry: NewEntry }) {
  const t = useTranslations('ios');
  const locale = useLocale();
  const { app, best } = entry;
  const icon = iconAt(app.iconUrl, 100);
  const source = [
    countryName(best.country, locale),
    t(`chart_${best.chart}` as never),
    ...(best.genre && best.genre !== 'all' ? [t(`g_${best.genre}` as never)] : []),
  ].join(' · ');
  return (
    <li className="border-b border-rule last:border-b-0">
      <Link
        href={appPath(app.id)}
        className="grid grid-cols-[32px_44px_1fr_auto] items-center gap-x-3 gap-y-1 px-1 py-2.5 transition-colors hover:bg-sunk sm:gap-x-4 sm:px-3"
      >
        <span className="row-span-2 text-[15px] font-semibold tabular-nums leading-none text-ink-3">{entry.rank}</span>
        {icon ? (
          // Apple's CDN: icons are linked, never stored (docs/MOBILE-APPS-DATA.md §8).
          <img
            src={icon}
            alt=""
            loading="lazy"
            width={44}
            height={44}
            className="row-span-2 h-11 w-11 rounded-[10px] border border-rule"
          />
        ) : (
          <span aria-hidden="true" className="row-span-2 h-11 w-11 rounded-[10px] bg-sunk" />
        )}
        <span className="min-w-0">
          <span className="block truncate text-[14.5px] font-medium text-ink">{app.name}</span>
          <span className="block truncate text-sr-caption text-ink-3">{app.developer}</span>
        </span>
        <span className="flex flex-col items-end gap-0.5 text-right">
          {app.ratings > 0 ? (
            <span className="text-sr-caption tabular-nums text-ink-3">
              ★ {app.rating.toFixed(1)} ({fmtInt(app.ratings)})
            </span>
          ) : (
            <span className="text-sr-caption text-ink-3">{t('noRatings')}</span>
          )}
          <span className="text-sr-caption tabular-nums text-ink-3">{formatPrice(app.price, app.currency, locale, t('free'))}</span>
        </span>
        {/* Where the place comes from, the point of the row: under both
            columns and free to wrap, so the rank is never cut off. */}
        <span className="col-span-2 text-sr-caption text-ink-2">
          {source} <span className="whitespace-nowrap font-semibold tabular-nums text-ink">#{best.rank}</span>
          {entry.charts > 1 && <span className="whitespace-nowrap text-ink-3"> · {t('newOnCharts', { n: entry.charts })}</span>}
          <span className="whitespace-nowrap tabular-nums text-ink-3"> · {t('newReleased', { day: entry.releasedAt.slice(0, 10) })}</span>
        </span>
      </Link>
    </li>
  );
}
