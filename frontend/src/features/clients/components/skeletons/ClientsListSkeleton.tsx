import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Loading placeholder shaped like the loaded list at each width: the card grid
 * below `xl` (the grid classes `ClientGrid` uses), the table from `xl` up. Both
 * are in the tree and switched with CSS, which is enough for a placeholder.
 */
export function ClientsListSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:hidden">
        {Array.from({ length: 5 }, (_, i) => (
          <ClientCardSkeleton key={i} />
        ))}
      </div>
      <div className="hidden xl:block">
        <SkeletonTable live={false} />
      </div>
    </div>
  );
}

/**
 * Mirrors `ClientCard`: same border, padding and radius, and each bar sits in a
 * box as tall as the text line it stands for, so the card is ~134px like the
 * loaded one.
 */
function ClientCardSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="border-border-subtle bg-bg-subtle flex flex-col items-stretch rounded-2xl border p-4"
    >
      <div className="flex h-5 items-center">
        <Skeleton className="h-4 w-36 rounded-lg" />
      </div>
      <div className="mt-0.5 flex h-4 items-center">
        <Skeleton className="h-3 w-24 rounded-lg" />
      </div>
      <div className="mt-4 grid grid-cols-2 gap-3">
        {[0, 1].map((i) => (
          <div key={i} className="min-w-0">
            <div className="flex h-3.5 items-center">
              <Skeleton className="h-2.5 w-14 rounded-lg" />
            </div>
            <div className="mt-1 flex h-7 items-center">
              <Skeleton className="h-5 w-12 rounded-lg" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
