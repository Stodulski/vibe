import { SkeletonCourtCard } from '@/shared/components/common/Skeletons';
import { CourtTableSkeleton } from './CourtTableSkeleton';

/**
 * Loading shape of the courts list. Same breakpoint as `CourtGrid`: cards
 * below `xl` (two columns from `lg`), the dense table from `xl` up. CSS-only,
 * so nothing is mounted twice.
 */
export function CourtListSkeleton() {
  return (
    <>
      <div className="grid w-full grid-cols-1 gap-3 lg:grid-cols-2 xl:hidden">
        <SkeletonCourtCard />
        <SkeletonCourtCard />
        <SkeletonCourtCard />
        <SkeletonCourtCard />
      </div>
      <div className="hidden xl:block">
        <CourtTableSkeleton />
      </div>
    </>
  );
}
