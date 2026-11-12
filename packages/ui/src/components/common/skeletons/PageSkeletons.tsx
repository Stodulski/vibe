import { Panel } from '../Panel';
import { Skeleton } from '../../ui/skeleton';
import { messages } from '../../../messages';
import { SkeletonStat, SkeletonTable } from './SkeletonPrimitives';

export function SkeletonPage() {
  return (
    <div className="animate-fade-in flex flex-col gap-12" role="status" aria-label={messages.loading}>
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

/** The unboxed, `max-w-xl` "complete your profile" checklist: progress header and bar, next step, disclosure. */
function ProfileChecklistSkeleton() {
  return (
    <div className="flex max-w-xl flex-col gap-4">
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

/**
 * Desktop-only trends block, mirrors DashboardContent's `hidden md:flex` block.
 * Heights are the measured loaded ones at `xl` (revenue + insights row ~610px,
 * occupancy heatmap ~714px); below `xl` the row stacks, so each card keeps a
 * chart-sized floor instead.
 */
function TrendsSkeleton() {
  return (
    <div className="mt-8 hidden md:flex md:flex-col md:gap-4 lg:gap-6">
      <Skeleton className="h-4 w-20 rounded-lg" />
      <div className="grid grid-cols-1 gap-4 md:gap-6 xl:grid-cols-5">
        <div className="border-border-subtle bg-bg-subtle flex rounded-2xl border p-4 sm:p-5 xl:col-span-3 xl:min-h-[600px]">
          <Skeleton className="min-h-72 w-full flex-1 rounded-xl" />
        </div>
        <div className="border-border-subtle bg-bg-subtle flex rounded-2xl border p-4 sm:p-5 xl:col-span-2 xl:min-h-[600px]">
          <Skeleton className="min-h-72 w-full flex-1 rounded-xl" />
        </div>
      </div>
      <div className="border-border-subtle bg-bg-subtle flex min-h-[420px] rounded-2xl border p-4 sm:p-5 xl:min-h-[700px]">
        <Skeleton className="min-h-[300px] w-full flex-1 rounded-xl" />
      </div>
    </div>
  );
}

/**
 * Payment overview next to the today-bookings panel. Floors are the measured
 * loaded heights: payment card ~328px stacked, both panels ~368px side by side.
 */
function OperationsSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-4 md:gap-6 lg:grid-cols-2">
      <Panel
        as="section"
        size="sm"
        mobile="card"
        className="flex h-full min-h-[328px] flex-col gap-4 p-4 lg:min-h-[368px]"
      >
        <Skeleton className="h-8 w-40 rounded-lg" />
        <Skeleton className="h-16 w-full rounded-xl" />
        <Skeleton className="h-4 w-full rounded-lg" />
        <Skeleton className="h-4 w-3/4 rounded-lg" />
      </Panel>
      <Panel as="section" size="sm" className="flex h-full min-h-[22rem] flex-col gap-2 py-4 sm:p-4 lg:min-h-[368px]">
        <Skeleton className="mb-1 h-5 w-40 rounded-lg" />
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-xl" />
        ))}
      </Panel>
    </div>
  );
}

/**
 * Matches `DashboardPage` + `DashboardContent` in the order and approximate
 * height of what loads: the mobile-only page title and the public-link bar,
 * the unboxed `max-w-xl` profile checklist (low-stock renders nothing while
 * loading, so it has no placeholder), the solid payment card next to the
 * today-bookings panel, then the `hidden md:flex` trends block.
 */
export function SkeletonDashboard() {
  return (
    <div className="animate-fade-in" role="status" aria-label={messages.loading}>
      {/* Page header: title is mobile-only (the masthead carries it from `md`), then the public-link bar */}
      <Skeleton className="mb-3 h-7 w-40 rounded-lg sm:mb-5 md:hidden" />
      <div className="mb-3 flex items-center gap-2 sm:mb-4">
        <Skeleton className="h-4 w-48 rounded-lg" />
        <Skeleton className="h-8 w-9 rounded-full sm:w-28" />
      </div>

      <div className="flex flex-col gap-4 md:gap-6">
        <ProfileChecklistSkeleton />

        <OperationsSkeleton />

        <TrendsSkeleton />
      </div>
    </div>
  );
}

/** One labelled input: label line, then the control. */
function FieldSkeleton({ tall = false }: { tall?: boolean }) {
  return (
    <div className="space-y-2">
      <Skeleton className="h-4 w-28 rounded-lg" />
      <Skeleton className={tall ? 'h-24 w-full rounded-lg' : 'h-10 w-full rounded-lg'} />
    </div>
  );
}

/**
 * Stand-in for the General tab's form (name, slug, description, address,
 * contact, amenities, schedule). The floor is the measured loaded height:
 * ~1650px at 320 (single column) and a little less in the 39rem card.
 */
function SettingsFormSkeleton() {
  return (
    <div className="lg:border-border-subtle lg:bg-bg-subtle flex min-h-[1650px] flex-col lg:min-h-[1500px] lg:max-w-[39rem] lg:rounded-2xl lg:border lg:p-6">
      <Skeleton className="mb-1 h-4 w-24 rounded-lg" />
      <Skeleton className="mb-6 h-3 w-48 rounded-lg" />
      <div className="space-y-6">
        <FieldSkeleton />
        <FieldSkeleton />
        <FieldSkeleton tall />
        <FieldSkeleton />
        <FieldSkeleton />
        <FieldSkeleton />
        <div className="flex flex-wrap gap-2">
          {Array.from({ length: 8 }, (_, i) => (
            <Skeleton key={i} className="h-9 w-24 rounded-full" />
          ))}
        </div>
        {Array.from({ length: 7 }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-lg" />
        ))}
        <Skeleton className="h-10 w-32 rounded-lg" />
      </div>
    </div>
  );
}

/**
 * Settings route skeleton. Mirrors `SettingsPage`: the page title is
 * mobile-only (the masthead carries it from `md`), three section tabs as a
 * `SectionNav` list from `lg` and an underlined `SectionTabs` strip below it,
 * and a content panel that is a card from `lg` and bare below.
 */
export function SkeletonSettings() {
  return (
    <div className="animate-fade-in" role="status" aria-label={messages.loading}>
      <Skeleton className="mb-3 h-7 w-36 rounded-lg sm:mb-5 md:hidden" />
      <div className="flex flex-col gap-3 sm:gap-4 lg:flex-row">
        {/* Section list — desktop */}
        <nav className="hidden w-52 shrink-0 lg:block">
          <div className="space-y-1">
            {Array.from({ length: 3 }, (_, i) => (
              <div key={i} className="flex h-12 items-center gap-3 rounded-xl px-3">
                <Skeleton className="size-4 rounded" />
                <Skeleton className="h-4 w-24 rounded-lg" />
              </div>
            ))}
          </div>
        </nav>
        {/* Section tabs — mobile: full-bleed underline strip, like `SectionTabs` */}
        <div className="-mx-3 px-3 sm:-mx-6 sm:px-6 lg:hidden">
          <div className="flex">
            {Array.from({ length: 3 }, (_, i) => (
              <div key={i} className="flex h-12 flex-1 items-center justify-center border-b-2 border-transparent px-2">
                <Skeleton className="h-4 w-16 rounded-lg" />
              </div>
            ))}
          </div>
        </div>
        {/* Content — a card from `lg`, bare below it, matching `SettingsPage`'s own content wrapper. */}
        <div className="min-w-0 flex-1">
          <SettingsFormSkeleton />
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
    <div className="animate-fade-in" role="status" aria-label={messages.loading}>
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
