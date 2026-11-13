import { Skeleton } from '../../ui/skeleton';
import { messages } from '../../../messages';
import { SkeletonStat, SkeletonTable } from './SkeletonPrimitives';

export function SkeletonPage() {
  return (
    <div className="flex flex-col gap-12" role="status" aria-label={messages.loading}>
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="mb-4 h-7 w-40 rounded-lg" />
          <Skeleton className="h-4 w-64 rounded-lg" />
        </div>
        <Skeleton className="h-10 w-32 rounded-lg" />
      </div>
      <div className="grid gap-4 sm:grid-cols-2 sm:gap-6 lg:grid-cols-4">
        <SkeletonStat />
        <SkeletonStat />
        <SkeletonStat />
        <SkeletonStat />
      </div>
      <SkeletonTable live={false} />
    </div>
  );
}
