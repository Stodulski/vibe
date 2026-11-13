import { Skeleton } from '@/shared/components/ui/skeleton';
import { PUBLIC_LINK_BAR_CLASS } from '../../components/dashboardLayout';

/** Loading stand-in for `PublicLinkBar`: the URL and the copy button. */
export function PublicLinkBarSkeleton() {
  return (
    <div className={PUBLIC_LINK_BAR_CLASS}>
      <Skeleton className="h-4 w-48 rounded-lg" />
      <Skeleton className="h-8 w-9 rounded-full sm:w-28" />
    </div>
  );
}
