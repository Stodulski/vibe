import { Skeleton } from '@/shared/components/ui/skeleton';
import { BACK_LINK_CLASS } from './backLinkLayout';

/** Loading stand-in for `BackLink`. */
export function BackLinkSkeleton() {
  return (
    <div className={BACK_LINK_CLASS}>
      <Skeleton className="size-4 rounded" />
      <Skeleton className="h-4 w-28 rounded-lg" />
    </div>
  );
}
