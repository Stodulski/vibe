import { Wrench } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Stand-in for the superadmin area while it is rebuilt from scratch. It is
 * rendered inside `AdminLayout`, so a superadmin who lands here after login
 * (RootRedirect) keeps the admin shell and its user menu.
 */
export function AdminPlaceholderPage() {
  usePageTitle(t.layout.adminPlaceholderTitle);
  return (
    <>
      <PageHeader title={t.layout.adminPlaceholderTitle} />
      <EmptyState
        icon={Wrench}
        title={t.layout.adminPlaceholderEmptyTitle}
        description={t.layout.adminPlaceholderDescription}
      />
    </>
  );
}
