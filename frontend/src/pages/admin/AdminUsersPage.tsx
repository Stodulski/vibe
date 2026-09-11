import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { UsersTable } from '@/features/admin';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export default function AdminUsersPage() {
  usePageTitle(t.admin.users.title);

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.admin.users.title} />
      <UsersTable />
    </div>
  );
}
