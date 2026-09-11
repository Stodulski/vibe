import { Skeleton } from '@/shared/components/ui/skeleton';

export function SkeletonComplexHeader() {
  return (
    <div className="rounded-2xl border border-border-subtle bg-bg-subtle">
      <div className="p-6 sm:p-7">
        <div className="flex items-start gap-5">
          <Skeleton className="size-14 shrink-0 rounded-2xl sm:size-16" />
          <div className="min-w-0 flex-1 space-y-2.5">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-64" />
          </div>
        </div>
        <div className="mt-4 flex flex-col gap-2 sm:mt-5 sm:flex-row sm:gap-x-5">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-4 w-36" />
        </div>
      </div>
    </div>
  );
}
