import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { ThemeToggle } from './ThemeToggle';
import { LocaleSwitcher } from './LocaleSwitcher';

export function Nav() {
  const t = useTranslations();
  const links = [
    { href: '/', label: t('nav.overview') },
    { href: '/trending', label: t('nav.trending') },
    { href: '/rising', label: t('nav.rising') },
    { href: '/categories', label: t('nav.categories') },
  ];
  return (
    <header className="sticky top-0 z-10 border-b border-border bg-bg/80 backdrop-blur">
      <nav className="mx-auto flex max-w-5xl items-center gap-6 px-4 py-3">
        <Link href="/" className="font-bold tracking-tight">
          {t('site.name')}
        </Link>
        <ul className="flex flex-1 items-center gap-4 text-sm">
          {links.slice(1).map((l) => (
            <li key={l.href}>
              <Link href={l.href} className="text-muted hover:text-fg">
                {l.label}
              </Link>
            </li>
          ))}
        </ul>
        <LocaleSwitcher />
        <ThemeToggle />
      </nav>
    </header>
  );
}
