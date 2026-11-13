import { AlertTriangle, Users } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { ClientInsights } from '@/shared/types/api.types';
import { useDashboardClientDetail } from '../hooks/useDashboardClientDetail';
import { ClientInsightsCardSkeleton } from './ClientInsightsCardSkeleton';
import { TREND_CARD_FILL_CLASS } from './dashboardLayout';
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

function ClientInsightsCard({ data, isLoading, isError, onRetry, complexId }: ClientInsightsCardProps) {
  const detail = useDashboardClientDetail(complexId);

  if (isError) {
    return (
      <section className={cn(TREND_CARD_FILL_CLASS, 'items-center justify-center gap-2 text-center')}>
        <AlertTriangle className="text-error-icon size-5" aria-hidden="true" />
        <p className="text-text-tertiary text-sm">{t.common.loadError}</p>
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t.layout.retry}
        </Button>
      </section>
    );
  }

  if (isLoading) return <ClientInsightsCardSkeleton />;

  if (!data) {
    return (
      <section className={TREND_CARD_FILL_CLASS}>
        <h3 className="text-text-primary mb-4 text-sm font-semibold">{t.dashboard.clientsTitle}</h3>
        <div className="flex flex-1 items-center justify-center">
          <EmptyState icon={Users} title={t.dashboard.noClientData} description="" />
        </div>
      </section>
    );
  }

  return (
    <section className={TREND_CARD_FILL_CLASS}>
      <h3 className="text-text-primary mb-4 text-sm font-semibold">{t.dashboard.clientsTitle}</h3>
      <ClientMetricsRow data={data} />
      <TopClientsList top={data.top} onSelect={detail.handleSelectTopClient} />
      <ClientInsightsDetailModals detail={detail} complexId={complexId} />
    </section>
  );
}

export default ClientInsightsCard;
