import { Skeleton } from '@/shared/components/ui/skeleton';
import { PaymentOverviewShell } from './PaymentOverviewShell';

/** Loading shape of `PaymentOverview`. The floor is the measured loaded height: ~328px stacked, ~368px beside `TodayBookings`. */
export function PaymentOverviewSkeleton() {
  return (
    <PaymentOverviewShell className="min-h-[328px] gap-4 lg:min-h-[368px]">
      <Skeleton className="h-8 w-40 rounded-lg" />
      <Skeleton className="h-16 w-full rounded-xl" />
      <Skeleton className="h-4 w-full rounded-lg" />
      <Skeleton className="h-4 w-3/4 rounded-lg" />
    </PaymentOverviewShell>
  );
}
