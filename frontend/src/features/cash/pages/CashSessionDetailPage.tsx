import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, AlertTriangle } from 'lucide-react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSessionDetail } from '../hooks/useCashSessionDetail';
import { ExpectedCashCard } from '../components/ExpectedCashCard';
import { MovementTotalsBreakdown } from '../components/MovementTotalsBreakdown';
import { BookingPaymentsBreakdown } from '../components/BookingPaymentsBreakdown';
import { MovementList } from '../components/MovementList';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';

const t = ES_AR;

/** Read-only — a closed session is immutable and never reopens, so this page never offers Ingreso/Egreso/Anular. */
export default function CashSessionDetailPage() {
  usePageTitle(t.cash.sessionDetailTitle);
  const { sessionId } = useParams<{ sessionId: string }>();
  const { selectedComplexId } = useSelectedComplex();
  const query = useCashSessionDetail(selectedComplexId, sessionId ?? null);

  if (!selectedComplexId) return null;

  return (
    <div className="animate-fade-in space-y-4">
      <Link
        to="/cash"
        className="text-text-tertiary hover:text-text-secondary flex items-center gap-1.5 text-sm transition-colors"
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {t.cash.backToCash}
      </Link>

      <PageHeader title={t.cash.sessionDetailTitle} />

      {query.isLoading ? (
        <SkeletonTable rows={4} />
      ) : !query.data ? (
        // Only the full-screen error when there is nothing cached — a
        // background refetch failure with `data` still around keeps
        // rendering it, with a non-blocking notice instead (T3 review).
        <EmptyState
          icon={AlertTriangle}
          title={t.cash.loadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={() => {
            void query.refetch();
          }}
        />
      ) : (
        <>
          {query.isError && (
            <StaleDataNotice
              onRetry={() => {
                void query.refetch();
              }}
            />
          )}
          <ExpectedCashCard session={query.data.cash_session} summary={query.data.summary} />
          <MovementTotalsBreakdown movementTotals={query.data.summary.movement_totals} />
          <BookingPaymentsBreakdown bookingPayments={query.data.summary.booking_payments} />
          <MovementList movements={query.data.movements} />
        </>
      )}
    </div>
  );
}
