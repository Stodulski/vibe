import { CourtCardSkeleton } from './CourtCardSkeleton';
import { CourtTableSkeleton } from './CourtTableSkeleton';
import { COURT_GRID_CLASS } from './courtLayout';

/**
 * Loading shape of `CourtGrid`. Same breakpoint: cards below `xl` (two
 * columns from `lg`), the dense table from `xl` up. CSS-only, so nothing is
 * mounted twice.
 */
export function CourtGridSkeleton() {
  return (
    <>
      <div className={`${COURT_GRID_CLASS} xl:hidden`}>
        {[0, 1, 2, 3].map((i) => (
          <CourtCardSkeleton key={i} />
        ))}
      </div>
      <div className="hidden xl:block">
        <CourtTableSkeleton />
      </div>
    </>
  );
}
