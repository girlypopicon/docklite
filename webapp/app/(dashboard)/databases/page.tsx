import { redirect } from 'next/navigation';

// Databases are containers now: they live on the Containers page with everything else. Old bookmarks and links land there.
export default function DatabasesPage() {
  redirect('/');
}
