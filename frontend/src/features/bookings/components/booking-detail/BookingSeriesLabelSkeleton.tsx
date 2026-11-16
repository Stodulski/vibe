import { Skeleton } from '@/shared/components/ui/skeleton';

/** Placeholder for `BookingSeriesLabel`: a label line over a value line, like an `InfoRow`. */
export function BookingSeriesLabelSkeleton() {
  return (
    <div className="space-y-1" aria-hidden="true">
      <Skeleton className="h-3 w-16 rounded" />
      <Skeleton className="h-5 w-56 max-w-full rounded" />
    </div>
  );
}
