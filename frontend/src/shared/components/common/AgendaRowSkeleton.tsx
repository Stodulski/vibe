import { Skeleton } from '@/shared/components/ui/skeleton';
import { AGENDA_ROW_LAYOUT_CLASS } from './agendaRowLayout';

/** Loading shape of `AgendaRow`: the day, then the kicker, the title and the meta line. */
export function AgendaRowSkeleton() {
  return (
    <div className={AGENDA_ROW_LAYOUT_CLASS} aria-hidden="true">
      <div className="flex shrink-0 items-center gap-4">
        <Skeleton className="h-5 w-28 rounded-lg sm:h-14 sm:w-14" />
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <Skeleton className="h-3 w-40 rounded-lg" />
        <Skeleton className="h-5 w-52 max-w-full rounded-lg" />
        <Skeleton className="h-4 w-64 max-w-full rounded-lg" />
      </div>
    </div>
  );
}
