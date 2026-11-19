import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { InstagramStoriesShell } from './InstagramStoriesShell';

const t = ES_AR;

function ToggleRowSkeleton() {
  return (
    <div className="space-y-3" aria-hidden="true">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-2">
          <Skeleton className="h-4 w-40 rounded-lg" />
          <Skeleton className="h-3 w-full max-w-md rounded-lg" />
        </div>
        <Skeleton className="h-5 w-9 shrink-0 rounded-full" />
      </div>
      <Skeleton className="h-10 w-full rounded-xl sm:w-48" />
    </div>
  );
}

/** The section while its settings load: two toggle rows and the save button, in the shell. */
export function InstagramStoriesSectionSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading}>
      <InstagramStoriesShell>
        <ToggleRowSkeleton />
        <ToggleRowSkeleton />
        <Skeleton className="h-9 w-full rounded-full sm:w-36" aria-hidden="true" />
      </InstagramStoriesShell>
    </div>
  );
}
