import { AlertTriangle, Users } from 'lucide-react';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { ClientInsights } from '@/shared/types/api.types';
import { useDashboardClientDetail } from '../hooks/useDashboardClientDetail';
import { ClientMetricsRow } from './client-insights/ClientMetricsRow';
import { TopClientsList } from './client-insights/TopClientsList';
import { ClientInsightsDetailModals } from './client-insights/ClientInsightsDetailModals';

const t = ES_AR;

interface ClientInsightsCardProps {
  data: ClientInsights | undefined;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  complexId: string;
}

function ClientInsightsSkeleton() {
  return (
    <section className="flex h-full flex-col rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-5">
      <Skeleton className="mb-4 h-4 w-24" />

      <div className="mb-4 grid grid-cols-3 gap-2">
        {[0, 1, 2].map((i) => (
          <div key={i} className="rounded-lg bg-bg-base/40 px-2.5 py-2">
            <Skeleton className="mx-auto h-5 w-8" />
            <Skeleton className="mx-auto mt-1.5 h-3 w-12" />
          </div>
        ))}
      </div>

      <Skeleton className="mb-2 h-3 w-28" />
      <div className="space-y-1">
        {[0, 1, 2, 3, 4].map((i) => (
          <div key={i} className="flex items-center justify-between gap-2 px-2 py-2.5">
            <div className="flex min-w-0 items-center gap-2">
              <Skeleton className="size-5 shrink-0 rounded-full" />
              <Skeleton className="h-3 w-24" />
            </div>
            <div className="flex shrink-0 items-center gap-3">
              <Skeleton className="h-3 w-10" />
              <Skeleton className="h-3 w-12" />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

export function ClientInsightsCard({ data, isLoading, isError, onRetry, complexId }: ClientInsightsCardProps) {
  const detail = useDashboardClientDetail(complexId);

  if (isError) {
    return (
      <section className="flex h-full flex-col items-center justify-center gap-2 rounded-2xl border border-border-subtle bg-bg-subtle p-4 text-center sm:p-5">
        <AlertTriangle className="size-5 text-error-icon" aria-hidden="true" />
        <p className="text-sm text-text-tertiary">{t.common.loadError}</p>
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t.layout.retry}
        </Button>
      </section>
    );
  }

  if (isLoading) return <ClientInsightsSkeleton />;

  if (!data) {
    return (
      <section className="flex h-full flex-col rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-5">
        <h3 className="mb-4 text-sm font-semibold text-text-primary">{t.dashboard.clientsTitle}</h3>
        <div className="flex flex-1 items-center justify-center">
          <EmptyState icon={Users} title={t.dashboard.noClientData} description="" />
        </div>
      </section>
    );
  }

  return (
    <section className="flex h-full flex-col rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-5">
      <h3 className="mb-4 text-sm font-semibold text-text-primary">{t.dashboard.clientsTitle}</h3>
      <ClientMetricsRow data={data} />
      <TopClientsList top={data.top} onSelect={detail.handleSelectTopClient} />
      <ClientInsightsDetailModals detail={detail} complexId={complexId} />
    </section>
  );
}

export default ClientInsightsCard;
