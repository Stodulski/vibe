import { Skeleton } from '@/shared/components/ui/skeleton';
import { cn } from '@/shared/lib/utils';
import { KIT_ACTIONS_CLASS, KIT_ROW_CLASS, KIT_THUMB_CLASS } from './kitLayout';

/** Loading stand-in for `KitPieceRow`: the same card, with the preview, title and buttons as bars. */
export function KitPieceRowSkeleton() {
  return (
    <div className={KIT_ROW_CLASS} aria-hidden="true">
      <Skeleton className={cn(KIT_THUMB_CLASS, 'rounded-xl')} />
      <div className="flex flex-col gap-3 px-1 pb-1">
        <Skeleton className="h-7 w-40 max-w-full rounded-lg" />
        <div className={KIT_ACTIONS_CLASS}>
          <Skeleton className="h-9 w-full rounded-full" />
          <Skeleton className="h-9 w-full rounded-full" />
        </div>
      </div>
    </div>
  );
}
