import { AlertTriangle } from 'lucide-react';
import { CASH_STACK_CLASS } from '../../components/cashLayout';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ExpectedCashCard } from '../../components/ExpectedCashCard';
import { BookingPaymentsBreakdown } from '../../components/BookingPaymentsBreakdown';
import { CashSummarySkeleton } from '../../components/CashSummarySkeleton';
import { MovementList } from '../../components/MovementList';
import { CashActionButtons } from '../../components/CashActionButtons';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { OpenCashDialogs } from './OpenCashDialogs';
import type { useCashPage } from './useCashPage';
import type { CashMovement } from '@/shared/types/api.types';

interface OpenCashViewProps {
  complexId: string;
  sessionId: string;
  detailQuery: ReturnType<typeof useCashPage>['detailQuery'];
  movementDialog: 'income' | 'expense' | null;
  onOpenMovementDialog: (kind: 'income' | 'expense') => void;
  onCloseMovementDialog: () => void;
  onOpenCloseDialog: () => void;
  voidTarget: CashMovement | null;
  onVoidTarget: (movement: CashMovement) => void;
  onClearVoidTarget: () => void;
}

const t = ES_AR;

export function OpenCashView({
  complexId,
  sessionId,
  detailQuery,
  movementDialog,
  onOpenMovementDialog,
  onCloseMovementDialog,
  onOpenCloseDialog,
  voidTarget,
  onVoidTarget,
  onClearVoidTarget,
}: OpenCashViewProps) {
  if (detailQuery.isLoading) return <CashSummarySkeleton />;

  // Only the full-screen error when there is nothing cached to show — a
  // background refetch failure with `data` still around keeps rendering the
  // summary/dialogs, with a non-blocking notice instead, so an open dialog or
  // typed input is never unmounted by a transient failure (T3 review).
  if (!detailQuery.data) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.cash.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={() => {
          void detailQuery.refetch();
        }}
      />
    );
  }

  const { cash_session: session, summary, movements } = detailQuery.data;

  return (
    <div className={CASH_STACK_CLASS}>
      {detailQuery.isError && (
        <StaleDataNotice
          onRetry={() => {
            void detailQuery.refetch();
          }}
        />
      )}

      <ExpectedCashCard session={session} summary={summary} movements={movements} />

      <CashActionButtons
        onIncome={() => {
          onOpenMovementDialog('income');
        }}
        onExpense={() => {
          onOpenMovementDialog('expense');
        }}
        onClose={onOpenCloseDialog}
      />

      <MovementList movements={movements} onVoid={onVoidTarget} />
      <BookingPaymentsBreakdown bookingPayments={summary.booking_payments} />

      <OpenCashDialogs
        complexId={complexId}
        sessionId={sessionId}
        movementDialog={movementDialog}
        onCloseMovementDialog={onCloseMovementDialog}
        voidTarget={voidTarget}
        onClearVoidTarget={onClearVoidTarget}
      />
    </div>
  );
}
