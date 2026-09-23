import { Link } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import { useCashSession } from '@/shared/hooks/useCashSession';

const t = ES_AR;

interface CashboxPanelProps {
  complexId: string;
}

function CashboxPanelSkeleton() {
  return (
    <Panel as="section" size="sm" className="flex h-full flex-col justify-center gap-2 p-4" aria-label={t.cash.title}>
      <Skeleton className="h-4 w-20" />
      <Skeleton className="h-7 w-32" />
      <Skeleton className="h-3 w-28" />
    </Panel>
  );
}

/**
 * The dashboard's actual cash control (pos-cashbox T6): open till -> expected
 * cash and when it opened; closed till -> "Abrir caja". `TodayRevenueHeader`
 * (in `PaymentOverview`) is booking revenue only and never answered this
 * question — that rename is what freed up "Caja" as this panel's name.
 *
 * Self-contained, the same shape `RevenueChart`/`OccupancyHeatmap` use: it
 * takes only `complexId` and owns its own query (`useCashSession`, shared
 * with the Caja screens) rather than threading cash-session props through
 * `DashboardContent`.
 */
export function CashboxPanel({ complexId }: CashboxPanelProps) {
  const query = useCashSession(complexId);
  const retry = () => {
    void query.refetch();
  };

  if (query.isLoading) return <CashboxPanelSkeleton />;

  // A real error with nothing cached to show — the same "compact retry, no
  // full-page takeover" shape ClientInsightsCard already uses for a sibling
  // dashboard card.
  if (query.isRealError && !query.data) {
    return (
      <Panel
        as="section"
        size="sm"
        className="flex h-full flex-col items-center justify-center gap-2 p-4 text-center"
        aria-label={t.cash.title}
      >
        <AlertTriangle className="text-error-icon size-5" aria-hidden="true" />
        <p className="text-text-tertiary text-sm">{t.common.loadError}</p>
        <Button variant="outline" size="sm" onClick={retry}>
          {t.layout.retry}
        </Button>
      </Panel>
    );
  }

  const data = query.data;

  return (
    <Panel as="section" size="sm" className="flex h-full flex-col p-4" aria-label={t.cash.title}>
      <h3 className="text-text-primary mb-2 text-sm font-semibold">{t.cash.title}</h3>

      {/* A background refetch failure while data from an earlier successful
          read is still on screen — keep rendering it, non-blocking notice on
          top, the same shape OpenCashView gives the Caja screen itself. */}
      {query.isRealError && <StaleDataNotice onRetry={retry} />}

      {data ? (
        <div className="flex flex-1 flex-col justify-center gap-1">
          <p className="text-text-secondary text-sm font-medium">{t.dashboard.cashboxOpen}</p>
          <p className="score-text text-text-primary text-lg font-bold">{formatPrice(data.summary.expected_cash)}</p>
          <p className="text-text-tertiary text-xs">
            {t.cash.openedAt}: {formatVenueDayTime(data.cash_session.opened_at)}
          </p>
          <Button asChild size="sm" variant="outline" className="mt-2 w-fit">
            <Link to="/cash">{t.dashboard.cashboxGoToShift}</Link>
          </Button>
        </div>
      ) : (
        <div className="flex flex-1 flex-col justify-center gap-3">
          <p className="text-text-secondary text-sm font-medium">{t.dashboard.cashboxClosed}</p>
          <Button asChild size="sm" className="w-fit">
            <Link to="/cash">{t.cash.openAction}</Link>
          </Button>
        </div>
      )}
    </Panel>
  );
}
