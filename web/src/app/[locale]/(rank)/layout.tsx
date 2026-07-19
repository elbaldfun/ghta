import { RankHeader } from '@/components/rank/Header';

// The approved 2a design: full-width shell with its own persistent header.
export default function RankLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <RankHeader />
      {children}
    </>
  );
}
