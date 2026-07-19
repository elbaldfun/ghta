import { Nav } from '@/components/Nav';

// Legacy multi-source pages keep their original header and narrow container.
export default function ClassicLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Nav />
      <main className="mx-auto max-w-5xl px-4 py-8">{children}</main>
    </>
  );
}
