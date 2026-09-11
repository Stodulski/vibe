import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { ComplexesTable } from '@/features/admin';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export default function AdminComplexesPage() {
  usePageTitle(t.admin.complexes.title);

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.admin.complexes.title} />
      <ComplexesTable />
    </div>
  );
}
