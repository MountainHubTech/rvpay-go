import { NextResponse } from 'next/server';

// Forces Next.js to evaluate this route dynamically on every request
export const dynamic = 'force-dynamic';

export async function GET() {
  return NextResponse.json({ status: 'ok' }, { status: 200 });
}