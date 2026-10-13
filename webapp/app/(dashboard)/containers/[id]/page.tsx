import ContainerPage from './ContainerPage';

// Pre-built ids let the public demo be exported as plain static files; any other id still renders on demand in the real app.
export function generateStaticParams() {
  return [{ id: 'demo' }];
}

export default function Page() {
  return <ContainerPage />;
}
