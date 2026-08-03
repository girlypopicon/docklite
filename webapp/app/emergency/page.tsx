import { notFound } from 'next/navigation';

export default function EmergencyPage() {
  if (process.env.ENABLE_DEBUG_PAGES !== 'true') {
    notFound();
  }

  return (
    <div style={{ padding: '50px', fontFamily: 'monospace', background: '#000', color: '#0ff' }}>
      <h1>EMERGENCY TEST</h1>
      <p>If you see this, basic Next.js works</p>
      <p>Status: ALIVE</p>
    </div>
  );
}
