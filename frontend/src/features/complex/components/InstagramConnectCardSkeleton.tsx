import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

// The not-connected layout, the one most owners see first: the 32px status
// row, the description and the professional-account note (16px lines, 12px
// apart), and a 36px button after 8px of padding, in the card's `space-y-5`.
export function InstagramConnectCardSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className="space-y-5">
      <div className="flex items-center gap-2.5">
        <Skeleton className="size-8 shrink-0 rounded-full" />
        <div className="flex h-5 items-center">
          <Skeleton className="h-4 w-44 rounded-lg" />
        </div>
      </div>
      <div className="space-y-3">
        <div className="flex h-4 items-center">
          <Skeleton className="h-3 w-full max-w-md rounded-lg" />
        </div>
        <div className="flex h-4 items-center gap-2">
          <Skeleton className="size-3.5 shrink-0 rounded-full" />
          <Skeleton className="h-3 w-64 rounded-lg" />
        </div>
      </div>
      <div className="pt-2">
        <Skeleton className="h-9 w-full rounded-full sm:w-44" />
      </div>
    </div>
  );
}
