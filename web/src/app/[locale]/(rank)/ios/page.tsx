import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';
import { COUNTRY_COOKIE, chartPath, defaultCountry, isCountry } from '@/lib/api/ios';

// Decided per visitor (remembered storefront, else the interface language),
// so never cached: a temporary redirect to that storefront's free chart.
export const dynamic = 'force-dynamic';

export default function IOSIndex({ params: { locale } }: { params: { locale: string } }) {
  const remembered = cookies().get(COUNTRY_COOKIE)?.value ?? '';
  const country = isCountry(remembered) ? remembered : defaultCountry(locale);
  redirect(`/${locale}${chartPath(country, 'free')}`);
}
