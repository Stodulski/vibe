import { AlertTriangle } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ExpectedCashCard } from '../../components/ExpectedCashCard';
import { MovementTotalsBreakdown } from '../../components/MovementTotalsBreakdown';
import { BookingPaymentsBreakdown } from '../../components/BookingPaymentsBreakdown';
import { MovementList } from '../../components/MovementList';
import { CashActionButtons } from '../../components/CashActionButtons';
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
  closeDialogOpen: boolean;
  onOpenCloseDialog: () => void;
  onCloseCloseDialog: () => void;
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
  closeDialogOpen,
  onOpenCloseDialog,
  onCloseCloseDialog,
  voidTarget,
  onVoidTarget,
  onClearVoidTarget,
}: OpenCashViewProps) {
  if (detailQuery.isLoading) return <SkeletonTable rows={4} />;

  if (detailQuery.isError || !detailQuery.data) {
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
    <div className="space-y-4">
      <ExpectedCashCard session={session} summary={summary} />

      <CashActionButtons
        onIncome={() => {
          onOpenMovementDialog('income');
        }}
        onExpense={() => {
          onOpenMovementDialog('expense');
        }}
        onClose={onOpenCloseDialog}
      />

      <MovementTotalsBreakdown movementTotals={summary.movement_totals} />
      <BookingPaymentsBreakdown bookingPayments={summary.booking_payments} />
      <MovementList movements={movements} onVoid={onVoidTarget} />

      <OpenCashDialogs
        complexId={complexId}
        sessionId={sessionId}
        expectedCash={summary.expected_cash}
        movementDialog={movementDialog}
        onCloseMovementDialog={onCloseMovementDialog}
        closeDialogOpen={closeDialogOpen}
        onCloseCloseDialog={onCloseCloseDialog}
        voidTarget={voidTarget}
        onClearVoidTarget={onClearVoidTarget}
      />
    </div>
  );
}
