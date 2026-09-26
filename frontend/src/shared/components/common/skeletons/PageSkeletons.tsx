import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SkeletonStat, SkeletonTable } from './SkeletonPrimitives';

const t = ES_AR;

export function SkeletonPage() {
  return (
    <div className="animate-fade-in flex flex-col gap-12" role="status" aria-label={t.common.loading}>
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

/**
 * Matches `DashboardContent`'s own structure and surfaces exactly
 * (odd/tasks/app-dark-contrast.md T2 follow-up — this used to depict a
 * top stat-tile row and a "live courts" grid that no longer exist on the
 * page, and boxed both mobile sections that `Panel` keeps flat below `sm`):
 * a low-stock placeholder, the payment/today-bookings grid (both `Panel`,
 * flat below `sm`), then the `hidden md:flex` trends block (always boxed,
 * since it never renders below `md`).
 */
export function SkeletonDashboard() {
  return (
    <div className="animate-fade-in flex flex-col gap-4 md:gap-6" role="status" aria-label={t.common.loading}>
      {/* Low-stock alert */}
      <Panel as="section" size="sm" className="flex flex-col gap-2 py-4 sm:p-4">
        <Skeleton className="h-4 w-24 rounded-lg" />
        <Skeleton className="h-4 w-full rounded-lg" />
      </Panel>

      {/* Payment overview + Today bookings */}
      <div className="grid grid-cols-1 gap-4 md:gap-6 lg:grid-cols-2">
        <Panel as="section" size="sm" className="flex h-full flex-col gap-3 py-4 sm:p-4">
          <Skeleton className="h-8 w-28 rounded-lg" />
          <Skeleton className="h-20 w-full rounded-xl" />
        </Panel>
        <Panel as="section" size="sm" className="flex h-full flex-col gap-2 py-4 sm:p-4">
          <Skeleton className="mb-1 h-5 w-40 rounded-lg" />
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} className="h-14 w-full rounded-xl" />
          ))}
        </Panel>
      </div>

      {/* Trends — desktop only, mirrors DashboardContent's `hidden md:flex` block */}
      <div className="mt-8 hidden md:flex md:flex-col md:gap-4 lg:gap-6">
        <Skeleton className="h-4 w-20 rounded-lg" />
        <div className="grid grid-cols-1 gap-4 md:gap-6 xl:grid-cols-5">
          <div className="border-border-subtle bg-bg-subtle rounded-2xl border p-4 sm:p-5 xl:col-span-3">
            <Skeleton className="h-72 w-full rounded-xl" />
          </div>
          <div className="border-border-subtle bg-bg-subtle rounded-2xl border p-4 sm:p-5 xl:col-span-2">
            <Skeleton className="h-72 w-full rounded-xl" />
          </div>
        </div>
        <div className="border-border-subtle bg-bg-subtle rounded-2xl border p-4 sm:p-5">
          <Skeleton className="h-[300px] w-full rounded-xl" />
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
        {/* Content — a card from `lg`, bare below it, matching `SettingsPage`'s own content wrapper exactly (odd/tasks/app-dark-contrast.md T2 follow-up). */}
        <div className="min-w-0 flex-1">
          <div className="lg:border-border-subtle lg:bg-bg-subtle lg:max-w-[39rem] lg:rounded-2xl lg:border lg:p-6">
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

/**
 * Matches `BookingsPage`'s own structure exactly (odd/tasks/app-dark-contrast.md
 * T2 follow-up: the date strip and the calendar dropped their cards — a
 * hairline divider replaced the strip's card, and the calendar was never
 * boxed to begin with — and the "Filters" row here never had a real
 * counterpart on this page).
 */
export function SkeletonBookings() {
  return (
    <div className="animate-fade-in" role="status" aria-label={t.common.loading}>
      {/* Page header: title + create/block buttons */}
      <div className="mb-3 flex flex-col gap-2.5 sm:mb-5 sm:flex-row sm:items-center sm:justify-between">
        <Skeleton className="h-7 w-28 rounded-lg md:hidden" />
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row md:ml-auto">
          <Skeleton className="h-11 flex-1 rounded-xl sm:w-28 sm:flex-none" />
          <Skeleton className="h-11 flex-1 rounded-xl sm:w-28 sm:flex-none" />
        </div>
      </div>

      {/* Date navigation — no card, a hairline divider like the real strip */}
      <div className="border-border-subtle mb-6 border-b pb-6 sm:mb-8 sm:pb-8">
        <div className="flex items-center justify-center gap-2">
          <Skeleton className="size-8 rounded-lg" />
          <Skeleton className="h-6 w-44 rounded-lg" />
          <Skeleton className="size-8 rounded-lg" />
        </div>
        <div className="mt-3 flex items-center justify-between gap-0.5 sm:justify-center sm:gap-1">
          {Array.from({ length: 7 }, (_, i) => (
            <Skeleton key={i} className="h-14 flex-1 rounded-xl sm:w-14 sm:flex-none" />
          ))}
        </div>
      </div>

      {/* Calendar — `BookingCalendar`/`CourtTimeGrid` render no card at any width */}
      <div className="space-y-2">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-xl" />
        ))}
      </div>
    </div>
  );
}

/** Complex selector skeleton with complex cards */
export function SkeletonComplexSelector() {
  return (
    <div className="animate-fade-in w-full max-w-2xl" role="status" aria-label={t.common.loading}>
      <div className="mb-6 text-center sm:mb-8">
        <Skeleton className="mx-auto h-7 w-56 rounded-lg" />
        <Skeleton className="mx-auto mt-2 h-4 w-72 rounded-lg" />
      </div>
      <div className="grid grid-cols-1 gap-3 sm:gap-4">
        {Array.from({ length: 2 }, (_, i) => (
          <div key={i} className="border-border-subtle bg-bg-elevated rounded-2xl border p-4 sm:p-5">
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
