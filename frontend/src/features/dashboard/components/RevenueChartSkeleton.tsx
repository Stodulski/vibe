import { Skeleton } from '@/shared/components/ui/skeleton';
import { cn } from '@/shared/lib/utils';
import { TREND_CARD_CLASS } from './dashboardLayout';

/** Loading shape of `RevenueChart`: title and total, then the chart area. `className` lets a page pin a taller floor. */
export function RevenueChartSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn(TREND_CARD_CLASS, 'flex h-72 flex-col xl:h-full', className)} aria-hidden="true">
      <div className="mb-4 sm:mb-5">
        <Skeleton className="h-4 w-24 rounded-lg" />
      </div>
      <Skeleton className="min-h-0 w-full flex-1 rounded-xl" />
    </div>
  );
}
