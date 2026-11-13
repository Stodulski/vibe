import { Skeleton } from '@/shared/components/ui/skeleton';
import { PROFILE_CHECKLIST_CLASS } from './profileChecklistLayout';

/** Loading shape of `ProfileChecklist`: progress header and bar, next step, disclosure. */
export function ProfileChecklistSkeleton() {
  return (
    <div className={PROFILE_CHECKLIST_CLASS}>
      <div className="flex flex-col gap-2">
        <div className="flex items-baseline justify-between gap-2">
          <Skeleton className="h-4 w-36 rounded-lg" />
          <Skeleton className="h-4 w-16 rounded-lg" />
        </div>
        <Skeleton className="h-1.5 w-full rounded-full" />
      </div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
        <div className="flex flex-col gap-1.5">
          <Skeleton className="h-4 w-40 rounded-lg" />
          <Skeleton className="h-4 w-56 rounded-lg" />
        </div>
        <Skeleton className="h-8 w-full rounded-lg sm:w-28" />
      </div>
      <Skeleton className="-mt-2 h-8 w-32 rounded-lg" />
    </div>
  );
}
