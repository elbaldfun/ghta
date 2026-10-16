import { useTranslations } from 'next-intl';
import { PageTabs } from './PageTabs';

/**
 * The GitHub section's views. They used to be top-level nav items; the
 * header now has one GitHub entry and these sit under it.
 */
export function GithubTabs() {
  const t = useTranslations('rank');
  return (
    <PageTabs
      items={[
        { href: '/github', label: t('tabList') },
        { href: '/breakout', label: t('navBreakout') },
        { href: '/ai', label: t('navAi') },
        { href: '/ecosystem', label: t('navEcosystem') },
        { href: '/topics', label: t('navTopics') },
        { href: '/map', label: t('navMap') },
      ]}
    />
  );
}
