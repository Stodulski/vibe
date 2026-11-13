import { Skeleton } from '@/shared/components/ui/skeleton';
import { OccupancyHeatmapPanel } from './OccupancyHeatmapPanel';

/** Loading shape of `OccupancyHeatmap`. The floor is the measured loaded height (~714px at `xl`). */
export function OccupancyHeatmapSkeleton() {
  return (
    <OccupancyHeatmapPanel className="flex min-h-[420px] xl:min-h-[700px]" aria-hidden="true">
      <Skeleton className="min-h-[300px] w-full flex-1 rounded-xl" />
    </OccupancyHeatmapPanel>
  );
}
