import { redirect } from 'next/navigation';

// Databases are boxes now: they live on the Boxes page with everything else. Old bookmarks and links land there.
export default function DatabasesPage() {
  redirect('/');
}
