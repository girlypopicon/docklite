import DatabaseEditPage from './DatabaseEditPage';

// Pre-built ids let the public demo be exported as plain static files; any other id still renders on demand in the real app.
export function generateStaticParams() {
  return Array.from({ length: 60 }, (_, i) => ({ id: String(i + 1) }));
}

export default function Page() {
  return <DatabaseEditPage />;
}
