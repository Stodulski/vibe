import { Skeleton } from '@/shared/components/ui/skeleton';
import { SERIES_LABEL_ROW } from './seriesLabelLayout';

/** Placeholder for `BookingSeriesLabel`: same row height, a short line where the text will be. */
export function BookingSeriesLabelSkeleton() {
  return (
    <div className={SERIES_LABEL_ROW} aria-hidden="true">
      <Skeleton className="size-4 shrink-0 rounded" />
      <Skeleton className="h-4 w-56 max-w-full rounded" />
    </div>
  );
}
