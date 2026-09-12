import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Matches `ReportHeadline` + a handful of method rows so the panel keeps its
 * loaded height while the monthly report loads, instead of collapsing to a
 * centered spinner (UI-06).
 */
export function ReportCardSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading}>
      <div className="border-border-subtle mb-5 border-b pb-4">
        <Skeleton className="h-3 w-32 rounded-lg" />
        <Skeleton className="mt-2 h-8 w-40 rounded-lg" />
      </div>
      <div className="space-y-3">
        {Array.from({ length: 4 }, (_, i) => (
          <div key={i} className="border-border-subtle/50 flex items-center justify-between border-b pb-3">
            <Skeleton className="h-4 w-24 rounded-lg" />
            <div className="flex flex-col items-end gap-1">
              <Skeleton className="h-4 w-20 rounded-lg" />
              <Skeleton className="h-3 w-14 rounded-lg" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
