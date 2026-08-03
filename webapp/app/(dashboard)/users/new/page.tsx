import { redirect } from 'next/navigation';

export default function CreateUserPage() {
  redirect('/settings/users/new');
}
