import { Skeleton } from '@/shared/components/ui/skeleton';
import { cn } from '@/shared/lib/utils';
import { KIT_ACTIONS_CLASS, KIT_ROW_CLASS, KIT_THUMB_CLASS } from './kitLayout';

/** Loading stand-in for `KitPieceRow`: the same frame, with the thumbnail, words and buttons as bars. */
export function KitPieceRowSkeleton() {
  return (
    <div className={KIT_ROW_CLASS} aria-hidden="true">
      <Skeleton className={cn(KIT_THUMB_CLASS, 'rounded-xl')} />
      <div className="flex min-w-0 flex-1 flex-col justify-center gap-3">
        <div className="space-y-2">
          <Skeleton className="h-6 w-40 max-w-full rounded-lg" />
          <Skeleton className="h-4 w-full rounded-lg" />
        </div>
        <div className={KIT_ACTIONS_CLASS}>
          <Skeleton className="h-9 rounded-full sm:w-40" />
          <Skeleton className="h-9 rounded-full sm:w-36" />
        </div>
      </div>
    </div>
  );
}
