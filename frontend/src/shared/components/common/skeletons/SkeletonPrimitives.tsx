import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function SkeletonCard() {
  return (
    <div className="border-border-subtle bg-bg-subtle rounded-2xl border p-6 sm:p-8" aria-hidden="true">
      <Skeleton className="mb-4 h-4 w-1/3 rounded-lg" />
      <Skeleton className="mb-4 h-4 w-full rounded-lg" />
      <Skeleton className="mb-4 h-4 w-5/6 rounded-lg" />
      <Skeleton className="h-4 w-2/3 rounded-lg" />
    </div>
  );
}

export function SkeletonCourtCard() {
  return (
    <div className="border-border-subtle overflow-hidden rounded-2xl border" aria-hidden="true">
      <Skeleton className="h-1 w-full rounded-none" />
      <div className="p-4 sm:p-6">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <Skeleton className="h-5 w-2/5 rounded-lg" />
            <div className="mt-2 flex gap-1.5">
              <Skeleton className="h-5 w-14 rounded-full" />
              <Skeleton className="h-5 w-16 rounded-full" />
              <Skeleton className="h-4 w-12 rounded-lg" />
            </div>
          </div>
          <Skeleton className="h-5 w-10 rounded-full" />
        </div>
        <div className="bg-bg-base/60 mt-4 rounded-xl px-3 py-2.5">
          <Skeleton className="h-5 w-28 rounded-lg" />
        </div>
        <div className="mt-4 flex gap-2">
          <Skeleton className="h-8 flex-1 rounded-lg sm:w-20 sm:flex-initial" />
          <Skeleton className="h-8 flex-1 rounded-lg sm:w-20 sm:flex-initial" />
          <Skeleton className="ml-auto h-8 w-9 rounded-lg" />
        </div>
      </div>
    </div>
  );
}

interface SkeletonTableProps {
  rows?: number;
  /** Set to false when nested inside a parent that already owns the loading announcement. */
  live?: boolean;
}

export function SkeletonTable({ rows = 5, live = true }: SkeletonTableProps) {
  return (
    <div
      className="border-border-subtle bg-bg-subtle rounded-2xl border"
      role={live ? 'status' : undefined}
      aria-label={live ? t.common.loading : undefined}
      aria-hidden={live ? undefined : true}
    >
      <div className="border-border-subtle flex gap-4 border-b p-4">
        <Skeleton className="h-4 w-1/4 rounded-lg" />
        <Skeleton className="h-4 w-1/4 rounded-lg" />
        <Skeleton className="h-4 w-1/4 rounded-lg" />
        <Skeleton className="h-4 w-1/4 rounded-lg" />
      </div>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="border-border-subtle flex gap-4 border-b p-4 last:border-b-0">
          <Skeleton className="h-4 w-1/4 rounded-lg" />
          <Skeleton className="h-4 w-1/4 rounded-lg" />
          <Skeleton className="h-4 w-1/4 rounded-lg" />
          <Skeleton className="h-4 w-1/4 rounded-lg" />
        </div>
      ))}
    </div>
  );
}

export function SkeletonStat() {
  return (
    <div className="border-border-subtle bg-bg-subtle rounded-2xl border p-6 sm:p-8" aria-hidden="true">
      <div className="flex items-center justify-between">
        <Skeleton className="h-3 w-20 rounded-lg" />
        <Skeleton className="size-9 rounded-lg" />
      </div>
      <Skeleton className="mt-4 h-8 w-16 rounded-lg" />
    </div>
  );
}

export function SkeletonCalendar() {
  return (
    <div
      className="border-border-subtle bg-bg-subtle rounded-2xl border p-6 sm:p-8"
      role="status"
      aria-label={t.common.loading}
    >
      <div className="mb-6 flex items-center justify-between">
        <Skeleton className="h-5 w-32 rounded-lg" />
        <div className="flex gap-2">
          <Skeleton className="size-9 rounded-lg" />
          <Skeleton className="size-9 rounded-lg" />
        </div>
      </div>
      <div className="grid grid-cols-7 gap-2">
        {Array.from({ length: 7 }, (_, i) => (
          <Skeleton key={`h-${String(i)}`} className="h-4 w-full rounded-lg" />
        ))}
        {Array.from({ length: 35 }, (_, i) => (
          <Skeleton key={i} className="h-10 w-full rounded-lg" />
        ))}
      </div>
    </div>
  );
}
