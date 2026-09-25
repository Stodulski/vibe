import { AlertTriangle } from 'lucide-react';
import { Panel } from '@/shared/components/common/Panel';
import { Button } from '@/shared/components/ui/button';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { TillSectionSkeleton, OpenTillSection, ClosedTillSection } from './today-card/TillSection';
import { IncomeSection } from './today-card/IncomeSection';
import { MethodBreakdown } from './today-card/MethodBreakdown';
import type { DashboardStats } from '@/shared/types/api.types';

const t = ES_AR;

interface TodayCardProps {
  complexId: string;
  stats: DashboardStats;
}

/**
 * The dashboard's single "Hoy" card (odd/tasks/dashboard-today-card.md):
 * replaces the old CashboxPanel ("Caja") and PaymentOverview ("Ingresos de
 * hoy"), which used to show two figures that measured different things and
 * never reconciled with each other — a shift's till movements on one side, a
 * bookings-only revenue total on the other.
 *
 * The till status/expected-cash half is still `useCashSession`-backed, the
 * same shift-scoped query CashboxPanel used and the Caja screens share. The
 * income half is `stats.today_money`, the day-level (not shift-level) total
 * the backend now computes across booking payments and cash-till movements.
 * The two live side by side deliberately: one answers "what should be in the
 * till right now", the other "how much came in today" — see
 * DayMoneyTotals's own doc comment for why they are not the same number.
 */
export function TodayCard({ complexId, stats }: TodayCardProps) {
  const query = useCashSession(complexId);
  const retry = () => {
    void query.refetch();
  };

  const isOpen = !!query.data;
  // The status badge answers "open or closed", so it stays hidden while
  // loading and while a real error has left us with no cached session to
  // answer from — the same "don't state a fact we don't have" rule
  // CashboxPanel followed.
  const knowsTillStatus = !query.isLoading && !(query.isRealError && !query.data);

  return (
    <Panel as="section" size="sm" className="flex flex-col p-4" aria-label={t.dashboard.todayCardTitle}>
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-text-primary text-base font-semibold">{t.dashboard.todayCardTitle}</h2>
        {knowsTillStatus && (
          <span className={cn('text-sm font-semibold', isOpen ? 'text-success-text' : 'text-error-text')}>
            {isOpen ? t.dashboard.cashboxOpen : t.dashboard.cashboxClosed}
          </span>
        )}
      </div>

      {query.isRealError && query.data && <StaleDataNotice onRetry={retry} />}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="md:border-border-subtle md:border-r md:pr-4">
          {query.isLoading ? (
            <TillSectionSkeleton />
          ) : query.isRealError && !query.data ? (
            <div className="flex flex-col items-start gap-2">
              <p className="text-text-tertiary flex items-center gap-1.5 text-sm">
                <AlertTriangle className="text-error-icon size-4" aria-hidden="true" />
                {t.common.loadError}
              </p>
              <Button variant="outline" size="sm" onClick={retry}>
                {t.layout.retry}
              </Button>
            </div>
          ) : isOpen && query.data ? (
            <OpenTillSection summary={query.data.summary} openedAt={query.data.cash_session.opened_at} />
          ) : (
            <ClosedTillSection />
          )}
        </div>

        <div>
          <IncomeSection totals={stats.today_money} />
          <MethodBreakdown byMethod={stats.today_money.by_method} />
        </div>
      </div>
    </Panel>
  );
}
