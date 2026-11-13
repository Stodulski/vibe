import { Skeleton } from '@/shared/components/ui/skeleton';

/**
 * Route fallback for `/profile`. The page has no data wait of its own, so this
 * only covers the lazy chunk: mobile title, section tabs (below `lg`) or nav
 * (from `lg`), and the form block.
 */
export function ProfilePageSkeleton() {
  return (
    <div className="animate-fade-in">
      <Skeleton className="mb-3 h-7 w-32 rounded-lg sm:mb-5 md:hidden" />
      <div className="flex flex-col gap-3 sm:gap-4 lg:flex-row">
        <Skeleton className="h-11 w-full rounded-xl lg:hidden" />
        <div className="hidden w-52 shrink-0 space-y-1 lg:block">
          <Skeleton className="h-12 w-full rounded-xl" />
          <Skeleton className="h-12 w-full rounded-xl" />
        </div>
        <div className="min-w-0 flex-1 space-y-4 lg:max-w-[39rem]">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-16 w-full rounded-xl" />
          ))}
        </div>
      </div>
    </div>
  );
}
