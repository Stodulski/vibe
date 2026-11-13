import { Skeleton } from '@/shared/components/ui/skeleton';
import { TREND_CARD_FILL_CLASS } from './dashboardLayout';

/** Loading shape of `ClientInsightsCard`: title, the three metric tiles, then five top-client rows. */
export function ClientInsightsCardSkeleton() {
  return (
    <section className={TREND_CARD_FILL_CLASS}>
      <Skeleton className="mb-4 h-4 w-24" />

      <div className="mb-4 grid grid-cols-3 gap-2">
        {[0, 1, 2].map((i) => (
          <div key={i} className="bg-bg-base/40 rounded-lg px-2.5 py-2">
            <Skeleton className="mx-auto h-5 w-8" />
            <Skeleton className="mx-auto mt-1.5 h-3 w-12" />
          </div>
        ))}
      </div>

      <Skeleton className="mb-2 h-3 w-28" />
      <div className="space-y-1">
        {[0, 1, 2, 3, 4].map((i) => (
          <div key={i} className="flex items-center justify-between gap-2 px-2 py-2.5">
            <div className="flex min-w-0 items-center gap-2">
              <Skeleton className="size-5 shrink-0 rounded-full" />
              <Skeleton className="h-3 w-24" />
            </div>
            <div className="flex shrink-0 items-center gap-3">
              <Skeleton className="h-3 w-10" />
              <Skeleton className="h-3 w-12" />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
