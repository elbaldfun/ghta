import { RankHeader } from '@/components/rank/Header';
import { RankFooter } from '@/components/rank/Footer';

// The approved 2a design: full-bleed header/footer, main content centered in a
// max-width container so wide screens don't stretch cards edge to edge. The
// sticky-footer flex keeps the footer at the viewport bottom on short pages.
export default function RankLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <RankHeader />
      <main className="mx-auto w-full max-w-screen-xl flex-1">{children}</main>
      <RankFooter />
    </div>
  );
}
