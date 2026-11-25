import { useLocale, useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { RankMove } from '@/components/rank/RankMove';
import { appPath, countryName, formatPrice, iconAt, type ChartEntry } from '@/lib/api/ios';
import { fmtInt } from '@/components/ui';

/** One place on an App Store chart: Apple's rank, its move, the app and its rating. */
export function ChartRow({ entry, moveScope, showGenre }: { entry: ChartEntry; moveScope: string; showGenre: boolean }) {
  const t = useTranslations('ios');
  const locale = useLocale();
  const { app } = entry;
  const icon = iconAt(app.iconUrl, 100);
  return (
    <li className="border-b border-rule last:border-b-0">
      <Link
        href={appPath(app.id)}
        className="grid grid-cols-[40px_44px_1fr_auto] items-center gap-3 px-1 py-2.5 transition-colors hover:bg-sunk sm:gap-4 sm:px-3"
      >
        <span className="flex flex-col items-start gap-1">
          <span className="text-[17px] font-semibold tabular-nums leading-none tracking-[-0.02em]">{entry.rank}</span>
          <RankMove change={entry.change} isNew={entry.new} scope={moveScope} />
        </span>
        {icon ? (
          // Apple's CDN: icons are linked, never stored (docs/MOBILE-APPS-DATA.md §8).
          <img src={icon} alt="" loading="lazy" width={44} height={44} className="h-11 w-11 rounded-[10px] border border-rule" />
        ) : (
          <span aria-hidden="true" className="h-11 w-11 rounded-[10px] bg-sunk" />
        )}
        <span className="min-w-0">
          <span className="block truncate text-[14.5px] font-medium text-ink">{app.name}</span>
          <span className="block truncate text-sr-caption text-ink-3">
            {app.developer}
            {showGenre && app.genre ? ` · ${t(`g_${app.genre}` as never)}` : ''}
          </span>
        </span>
        <span className="flex flex-col items-end gap-0.5 text-right">
          {app.ratings > 0 ? (
            <span
              className="text-[13px] font-medium tabular-nums text-ink"
              title={app.homeCountry ? t('ratingIn', { country: countryName(app.homeCountry, locale) }) : undefined}
            >
              ★ {app.rating.toFixed(1)}
              <span className="ml-1 text-sr-caption text-ink-3">({fmtInt(app.ratings)})</span>
            </span>
          ) : (
            <span className="text-sr-caption text-ink-3">{t('noRatings')}</span>
          )}
          <span className="text-sr-caption tabular-nums text-ink-3">{formatPrice(app.price, app.currency, locale, t('free'))}</span>
        </span>
      </Link>
    </li>
  );
}
