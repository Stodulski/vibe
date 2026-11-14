import { Skeleton } from '@/shared/components/ui/skeleton';
import { SEGMENTED_TABS_CLASS } from '../lib/tournamentLayout';

/** Loading shape of `SegmentedTabs`: `count` buttons of a typical label. */
export function SegmentedTabsSkeleton({ count = 2 }: { count?: number }) {
  return (
    <div className={SEGMENTED_TABS_CLASS} aria-hidden="true">
      {Array.from({ length: count }, (_, i) => (
        <Skeleton key={i} className="h-8 w-24 shrink-0 rounded-xl" />
      ))}
    </div>
  );
}
