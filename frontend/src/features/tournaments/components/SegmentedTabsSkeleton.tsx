import { Skeleton } from '@/shared/components/ui/skeleton';
import { SEGMENTED_TABS_CLASS } from '../lib/tournamentLayout';
import { SEGMENT_SKELETON_CLASS } from './segmentedTabsLayout';

/** Loading shape of `SegmentedTabs`: `count` segments of the same height and widths. */
export function SegmentedTabsSkeleton({ count = 2 }: { count?: number }) {
  return (
    <div className={SEGMENTED_TABS_CLASS} aria-hidden="true">
      {Array.from({ length: count }, (_, i) => (
        <Skeleton key={i} className={SEGMENT_SKELETON_CLASS} />
      ))}
    </div>
  );
}
