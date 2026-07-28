import { RankHeader } from '@/components/rank/Header';
import { RankFooter } from '@/components/rank/Footer';

// The approved 2a design: full-bleed header/footer borders, content centered.
// Width + gutter live in <PageShell> (each page wraps itself), so page bodies,
// the header row and the footer row share left/right edges by construction.
// The sticky-footer flex keeps the footer at the viewport bottom on short pages.
export default function RankLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <RankHeader />
      <main className="w-full flex-1">{children}</main>
      <RankFooter />
    </div>
  );
}
