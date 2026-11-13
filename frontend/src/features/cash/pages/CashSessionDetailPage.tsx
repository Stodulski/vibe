import { useParams } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { BackLink } from '@/shared/components/common/BackLink';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSessionDetail } from '../hooks/useCashSessionDetail';
import { ExpectedCashCard } from '../components/ExpectedCashCard';
import { BookingPaymentsBreakdown } from '../components/BookingPaymentsBreakdown';
import { CashSummarySkeleton } from '../components/CashSummarySkeleton';
import { CASH_STACK_CLASS } from '../components/cashLayout';
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
    <div className={CASH_STACK_CLASS}>
      <BackLink to="/cash">{t.cash.backToCash}</BackLink>

      <PageHeader title={t.cash.sessionDetailTitle} />

      {query.isLoading ? (
        <CashSummarySkeleton showActions={false} />
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
          <ExpectedCashCard
            session={query.data.cash_session}
            summary={query.data.summary}
            movements={query.data.movements}
          />
          <BookingPaymentsBreakdown bookingPayments={query.data.summary.booking_payments} />
          <MovementList movements={query.data.movements} />
        </>
      )}
    </div>
  );
}
