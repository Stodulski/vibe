import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { CourtGridSkeleton } from '../components/CourtGridSkeleton';

/**
 * What `CourtsPage` draws while it loads, chrome included: the header (mobile
 * title, create button at every width) and the list.
 */
export function CourtsPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton actions={<Skeleton className="h-9 w-full rounded-xl sm:w-40 md:ml-auto" />} />
      <CourtGridSkeleton />
    </div>
  );
}
