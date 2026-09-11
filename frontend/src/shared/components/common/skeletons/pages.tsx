import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SkeletonStat, SkeletonTable } from './primitives';

const t = ES_AR;

export function SkeletonPage() {
  return (
    <div className="flex flex-col gap-12 animate-fade-in" role="status" aria-label={t.common.loading}>
      <div className="flex items-center justify-between">
        <div>
          <Skeleton className="mb-4 h-7 w-40 rounded-lg" />
          <Skeleton className="h-4 w-64 rounded-lg" />
        </div>
        <Skeleton className="h-10 w-32 rounded-lg" />
      </div>
      <div className="grid gap-4 sm:grid-cols-2 sm:gap-6 lg:grid-cols-4">
        <SkeletonStat />
        <SkeletonStat />
        <SkeletonStat />
        <SkeletonStat />
      </div>
      <SkeletonTable live={false} />
    </div>
  );
}

/** Full dashboard skeleton matching all sections */
export function SkeletonDashboard() {
  return (
    <div className="animate-fade-in" role="status" aria-label={t.common.loading}>
      <div className="flex flex-col gap-3 sm:gap-4">
        {/* Stats */}
        <div className="grid grid-cols-2 gap-2 sm:gap-3 lg:grid-cols-4">
          <SkeletonStat />
          <SkeletonStat />
          <SkeletonStat />
          <SkeletonStat />
        </div>
        {/* Live courts */}
        <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-6">
          <Skeleton className="mb-4 h-5 w-36 rounded-lg" />
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-20 rounded-xl" />
            ))}
          </div>
        </div>
        {/* Today bookings + Payment overview */}
        <div className="grid grid-cols-1 gap-3 sm:gap-4 lg:grid-cols-5">
          <div className="lg:col-span-3">
            <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-6">
              <Skeleton className="mb-4 h-5 w-40 rounded-lg" />
              {Array.from({ length: 3 }, (_, i) => (
                <Skeleton key={i} className="mb-2 h-14 w-full rounded-xl" />
              ))}
            </div>
          </div>
          <div className="lg:col-span-2">
            <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-6">
              <Skeleton className="mb-4 h-5 w-32 rounded-lg" />
              <Skeleton className="h-40 w-full rounded-xl" />
            </div>
          </div>
        </div>
        {/* Revenue chart + Client insights */}
        <div className="grid grid-cols-1 gap-3 sm:gap-4 lg:grid-cols-5">
          <div className="lg:col-span-3">
            <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-6">
              <Skeleton className="mb-4 h-5 w-28 rounded-lg" />
              <Skeleton className="h-[200px] w-full rounded-xl" />
            </div>
          </div>
          <div className="lg:col-span-2">
            <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-6">
              <Skeleton className="mb-4 h-5 w-24 rounded-lg" />
              <Skeleton className="h-40 w-full rounded-xl" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/** Settings page skeleton with sidebar tabs + content area */
export function SkeletonSettings() {
  return (
    <div className="animate-fade-in" role="status" aria-label={t.common.loading}>
      <div className="mb-4 sm:mb-6">
        <Skeleton className="h-7 w-36 rounded-lg" />
      </div>
      <div className="flex flex-col gap-3 sm:gap-4 lg:flex-row">
        {/* Sidebar — desktop */}
        <nav className="hidden w-56 shrink-0 lg:block">
          <div className="space-y-1">
            {Array.from({ length: 4 }, (_, i) => (
              <div key={i} className="flex items-center gap-3 rounded-xl px-3 py-2.5">
                <Skeleton className="size-8 rounded-lg" />
                <div className="min-w-0 flex-1">
                  <Skeleton className="mb-1 h-4 w-20 rounded-lg" />
                  <Skeleton className="h-3 w-28 rounded-lg" />
                </div>
              </div>
            ))}
          </div>
        </nav>
        {/* Mobile tabs */}
        <div className="grid grid-cols-4 gap-1.5 lg:hidden">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-14 rounded-lg" />
          ))}
        </div>
        {/* Content */}
        <div className="min-w-0 flex-1">
          <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-4 sm:p-6">
            <Skeleton className="mb-1 h-4 w-24 rounded-lg" />
            <Skeleton className="mb-6 h-3 w-48 rounded-lg" />
            <div className="space-y-4">
              <Skeleton className="h-10 w-full rounded-lg" />
              <Skeleton className="h-10 w-full rounded-lg" />
              <Skeleton className="h-10 w-full rounded-lg" />
              <Skeleton className="h-24 w-full rounded-lg" />
              <Skeleton className="h-10 w-32 rounded-lg" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/** Bookings page skeleton with date strip + calendar grid */
export function SkeletonBookings() {
  return (
    <div className="animate-fade-in" role="status" aria-label={t.common.loading}>
      {/* Page header */}
      <div className="mb-4 flex items-center justify-between sm:mb-6">
        <Skeleton className="h-7 w-28 rounded-lg" />
        <Skeleton className="h-9 w-28 rounded-lg" />
      </div>
      <div className="space-y-2.5 sm:space-y-3">
        {/* Date navigation */}
        <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-3 sm:p-4">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1">
              <Skeleton className="size-8 rounded-lg" />
              <Skeleton className="h-6 w-44 rounded-lg" />
              <Skeleton className="size-8 rounded-lg" />
            </div>
            <Skeleton className="h-8 w-16 rounded-lg" />
          </div>
          {/* Week strip */}
          <div className="mt-3 flex items-center justify-between gap-0.5 sm:justify-center sm:gap-1">
            {Array.from({ length: 7 }, (_, i) => (
              <Skeleton key={i} className="h-14 flex-1 rounded-xl sm:w-14 sm:flex-none" />
            ))}
          </div>
        </div>
        {/* Filters */}
        <div className="flex items-center gap-2">
          <Skeleton className="h-10 flex-1 rounded-lg" />
          <Skeleton className="h-10 w-[140px] rounded-lg" />
          <Skeleton className="h-9 w-9 rounded-lg" />
        </div>
        {/* Calendar grid */}
        <div className="rounded-2xl border border-border-subtle bg-bg-subtle p-3 sm:p-4">
          <div className="space-y-2">
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-xl" />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

/** Complex selector skeleton with complex cards */
export function SkeletonComplexSelector() {
  return (
    <div className="w-full max-w-2xl animate-fade-in" role="status" aria-label={t.common.loading}>
      <div className="mb-6 text-center sm:mb-8">
        <Skeleton className="mx-auto h-7 w-56 rounded-lg" />
        <Skeleton className="mx-auto mt-2 h-4 w-72 rounded-lg" />
      </div>
      <div className="grid grid-cols-1 gap-3 sm:gap-4">
        {Array.from({ length: 2 }, (_, i) => (
          <div key={i} className="rounded-2xl border border-border-subtle bg-bg-elevated p-4 sm:p-5">
            <div className="flex items-start gap-3">
              <Skeleton className="size-10 shrink-0 rounded-xl" />
              <div className="min-w-0 flex-1">
                <Skeleton className="h-4 w-40 rounded-lg" />
                <Skeleton className="mt-1.5 h-3 w-28 rounded-lg" />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
