import { Skeleton } from '@/shared/components/ui/skeleton';

/** Loading stand-in for `DateNavStrip`: the day controls, then a week of day tiles. */
export function DateNavStripSkeleton() {
  return (
    <>
      <div className="flex items-center justify-center gap-2">
        <Skeleton className="size-8 rounded-lg" />
        <Skeleton className="h-6 w-44 rounded-lg" />
        <Skeleton className="size-8 rounded-lg" />
      </div>
      <div className="mt-3 flex items-center justify-between gap-0.5 sm:justify-center sm:gap-1">
        {Array.from({ length: 7 }, (_, i) => (
          <Skeleton key={i} className="h-14 flex-1 rounded-xl sm:w-14 sm:flex-none" />
        ))}
      </div>
    </>
  );
}
