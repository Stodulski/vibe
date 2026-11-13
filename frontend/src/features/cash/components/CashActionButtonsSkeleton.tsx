import { Skeleton } from '@/shared/components/ui/skeleton';
import { CASH_ACTION_LAST_CLASS, CASH_ACTIONS_CLASS } from './cashLayout';

/** Loading shape of `CashActionButtons`. */
export function CashActionButtonsSkeleton() {
  return (
    <div className={CASH_ACTIONS_CLASS}>
      <Skeleton className="h-12 rounded-xl sm:w-32" />
      <Skeleton className="h-12 rounded-xl sm:w-32" />
      <Skeleton className={`${CASH_ACTION_LAST_CLASS} h-12 rounded-xl sm:w-32`} />
    </div>
  );
}
