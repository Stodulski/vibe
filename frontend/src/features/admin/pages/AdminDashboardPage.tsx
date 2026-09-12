import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { useAdminStats, AdminStatsCards } from '@/features/admin';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { AlertCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

function SkeletonStats() {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: 8 }).map((_, i) => (
        <Skeleton key={i} className="h-[76px] rounded-lg" />
      ))}
    </div>
  );
}

export default function AdminDashboardPage() {
  usePageTitle(t.admin.stats.title);
  const { data: stats, isLoading, isError, refetch } = useAdminStats();

  return (
    <div className="animate-fade-in">
      <PageHeader title={t.admin.stats.title} />

      {isError ? (
        <div className="border-border-subtle bg-bg-subtle flex flex-col items-center justify-center gap-3 rounded-lg border p-12">
          <AlertCircle className="text-text-tertiary size-8" />
          <p className="text-text-tertiary text-sm">{t.common.error}</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void refetch();
            }}
          >
            {t.common.refresh}
          </Button>
        </div>
      ) : isLoading || !stats ? (
        <SkeletonStats />
      ) : (
        <AdminStatsCards stats={stats} />
      )}
    </div>
  );
}
