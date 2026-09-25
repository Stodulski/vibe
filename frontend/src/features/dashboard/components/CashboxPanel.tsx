import { Link } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatPrice } from '@/shared/lib/utils';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import { useCashSession } from '@/shared/hooks/useCashSession';
import type { CashMovementTotal, CashSessionSummary } from '@/shared/types/api.types';

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

/** Manual movements of one kind across every method (sales included — they are income movements). */
function totalByKind(totals: CashMovementTotal[], kind: CashMovementTotal['kind']): number {
  return totals.filter((row) => row.kind === kind).reduce((sum, row) => sum + row.total, 0);
}

function CashboxStat({
  label,
  value,
  valueClassName = 'text-text-primary',
}: {
  label: string;
  value: number;
  valueClassName?: string;
}) {
  return (
    <div className="min-w-0">
      <dt className="text-text-tertiary truncate text-xs">{label}</dt>
      <dd className={cn('score-text text-sm font-semibold whitespace-nowrap', valueClassName)}>{formatPrice(value)}</dd>
    </div>
  );
}

/** The open-till body: the shift's four totals, then when it opened and the way in. */
function OpenCashboxSummary({ summary, openedAt }: { summary: CashSessionSummary; openedAt: string }) {
  return (
    <div className="flex flex-1 flex-col justify-center gap-3">
      <dl className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-4">
        <CashboxStat
          label={t.cash.expectedCash}
          value={summary.expected_cash}
          valueClassName="text-text-primary text-lg font-bold"
        />
        <CashboxStat label={t.cash.openingCashLabel} value={summary.opening_cash} />
        <CashboxStat
          label={t.dashboard.cashboxShiftIncome}
          value={totalByKind(summary.movement_totals, 'income')}
          valueClassName="text-success-text"
        />
        <CashboxStat
          label={t.dashboard.cashboxShiftExpense}
          value={totalByKind(summary.movement_totals, 'expense')}
          valueClassName="text-error-text"
        />
      </dl>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-text-tertiary text-xs">
          {t.cash.openedAt}: {formatVenueDayTime(openedAt)}
        </p>
        <Button asChild size="sm" variant="outline">
          <Link to="/cash">{t.dashboard.cashboxGoToShift}</Link>
        </Button>
      </div>
    </div>
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
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-text-primary text-sm font-semibold">{t.cash.title}</h3>
        <span className={cn('text-sm font-semibold', data ? 'text-success-text' : 'text-error-text')}>
          {data ? t.dashboard.cashboxOpen : t.dashboard.cashboxClosed}
        </span>
      </div>

      {/* A background refetch failure while data from an earlier successful
          read is still on screen — keep rendering it, non-blocking notice on
          top, the same shape OpenCashView gives the Caja screen itself. */}
      {query.isRealError && <StaleDataNotice onRetry={retry} />}

      {data ? (
        <OpenCashboxSummary summary={data.summary} openedAt={data.cash_session.opened_at} />
      ) : (
        <div className="flex flex-1 flex-col justify-center gap-3">
          <Button asChild size="sm" className="w-fit">
            <Link to="/cash">{t.cash.openAction}</Link>
          </Button>
        </div>
      )}
    </Panel>
  );
}
