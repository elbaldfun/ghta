import { NextResponse, type NextRequest } from 'next/server';

// Same-origin proxy for search autocomplete: the browser hits /api/suggest and
// this handler calls the backend server-side, so we don't need CORS on the API
// (the rest of the site also calls it only from the server).
const API =
  process.env.API_URL ||
  (process.env.NODE_ENV === 'production' ? 'https://api.starrank.dev' : 'http://localhost:3000');

export const dynamic = 'force-dynamic';

export async function GET(req: NextRequest) {
  const q = (req.nextUrl.searchParams.get('q') ?? '').trim();
  if (q.length < 2) return NextResponse.json({ data: [] });
  try {
    const res = await fetch(`${API}/search/suggest?q=${encodeURIComponent(q)}`, { cache: 'no-store' });
    if (!res.ok) return NextResponse.json({ data: [] });
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ data: [] });
  }
}
