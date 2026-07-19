import { RankHeader } from '@/components/rank/Header';

// The approved 2a design: full-bleed header, main content centered in a
// max-width container so wide screens don't stretch cards edge to edge.
export default function RankLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <RankHeader />
      <div className="mx-auto w-full max-w-screen-xl">{children}</div>
    </>
  );
}
