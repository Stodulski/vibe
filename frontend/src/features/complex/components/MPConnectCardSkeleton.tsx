import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

// Holds the status block's height while `mp/status` loads, so the fees table
// below does not drop when it arrives. Mirrors the loaded block: the same
// `space-y-5` rhythm, a 32px icon row, InfoSection's three lines (16/16/14px,
// 12px apart) and a 36px button after 8px of padding. Not connected is the
// layout used because the loaded state is not known yet (about 186px).
export function MPConnectCardSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className="space-y-5">
      <div className="flex items-center gap-2.5">
        <Skeleton className="size-8 shrink-0 rounded-full" />
        <div className="flex h-5 items-center">
          <Skeleton className="h-4 w-48 rounded-lg" />
        </div>
      </div>
      <div className="space-y-3">
        <div className="flex h-4 items-center">
          <Skeleton className="h-3 w-64 rounded-lg" />
        </div>
        <div className="flex h-4 items-center gap-2">
          <Skeleton className="size-3.5 shrink-0 rounded-full" />
          <Skeleton className="h-3 w-48 rounded-lg" />
        </div>
        <div className="flex h-3.5 items-center">
          <Skeleton className="h-2.5 w-56 rounded-lg" />
        </div>
      </div>
      <div className="pt-2">
        <Skeleton className="h-9 w-full rounded-full sm:w-28" />
      </div>
    </div>
  );
}
