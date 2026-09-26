import { Skeleton } from '@/shared/components/ui/skeleton';

/**
 * Matches `ComplexHeader`'s own surface exactly: the bordered card arrives at
 * `lg`, not `sm` — below that the cover banner and logo already fill the
 * width, so a card drawn under them would be a border just inside the screen
 * edge (odd/tasks/app-dark-contrast.md T2 follow-up).
 */
export function SkeletonComplexHeader() {
  return (
    <div className="lg:border-border-subtle lg:bg-bg-subtle relative lg:rounded-2xl lg:border lg:p-7">
      <Skeleton className="aspect-[16/9] w-full rounded-xl" />
      <div className="relative pb-5 sm:pb-7">
        <div className="-mt-10 mb-4 ml-4 sm:-mt-12">
          <Skeleton className="size-20 rounded-2xl sm:size-24" />
        </div>
        <Skeleton className="h-6 w-48" />
        <div className="mt-2.5 flex flex-col gap-2 sm:flex-row sm:gap-x-5">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-4 w-36" />
        </div>
      </div>
    </div>
  );
}
