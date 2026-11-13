import { ES_AR } from '@/shared/i18n/es_AR';
import { CashActionButtonsSkeleton } from './CashActionButtonsSkeleton';
import { ExpectedCashCardSkeleton } from './ExpectedCashCardSkeleton';
import { MovementListSkeleton } from './MovementListSkeleton';
import { CASH_STACK_CLASS } from './cashLayout';

const t = ES_AR;

interface CashSummarySkeletonProps {
  /** Ingreso / Egreso / Cerrar caja row — the open shift has it, a closed session's detail page does not. */
  showActions?: boolean;
}

/** Loading stand-in for the open shift and for a closed session's detail: summary card, optional actions, movements. */
export function CashSummarySkeleton({ showActions = true }: CashSummarySkeletonProps) {
  return (
    <div role="status" aria-label={t.common.loading} className={CASH_STACK_CLASS}>
      <ExpectedCashCardSkeleton />
      {showActions && <CashActionButtonsSkeleton />}
      <MovementListSkeleton rows={4} />
    </div>
  );
}
