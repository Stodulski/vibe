import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** Cancel booking info skeleton */
export function SkeletonCancelInfo() {
  return (
    <div
      className="animate-fade-in mx-auto flex w-full max-w-md flex-col items-center gap-4 px-4 py-16"
      role="status"
      aria-label={t.common.loading}
    >
      <Skeleton className="size-16 rounded-full" />
      <Skeleton className="h-6 w-48 rounded-lg" />
      <Skeleton className="h-4 w-64 rounded-lg" />
      {/* Matches `BookingInfoCard`'s own surface: flat below `sm`, boxed
          from `sm:` up (odd/tasks/app-dark-contrast.md T2 follow-up). */}
      <div className="sm:border-border-subtle sm:bg-bg-subtle w-full sm:rounded-2xl sm:border sm:p-5">
        <div className="space-y-2">
          {Array.from({ length: 4 }, (_, i) => (
            <div key={i} className="flex justify-between">
              <Skeleton className="h-4 w-20 rounded-lg" />
              <Skeleton className="h-4 w-28 rounded-lg" />
            </div>
          ))}
        </div>
      </div>
      <Skeleton className="h-10 w-full rounded-xl" />
    </div>
  );
}

/** Public booking confirm page skeleton */
export function SkeletonBookConfirm() {
  return (
    <div className="animate-fade-in w-full space-y-6 sm:space-y-8" role="status" aria-label={t.common.loading}>
      {/* Step indicator */}
      <div className="flex items-center justify-center gap-3">
        {Array.from({ length: 3 }, (_, i) => (
          <div key={i} className="flex items-center gap-3">
            <Skeleton className="size-8 rounded-full" />
            {i < 2 && <Skeleton className="h-0.5 w-10 rounded-full" />}
          </div>
        ))}
      </div>
      {/* Back button */}
      <Skeleton className="h-5 w-36 rounded-lg" />
      {/* `BookingForm` renders no summary card — just the fields below — so
          this doesn't invent one either (odd/tasks/app-dark-contrast.md T2
          follow-up: a stale "summary card" section here used to show a box
          the real page never has). */}
      {/* Form fields */}
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <Skeleton className="h-10 rounded-lg" />
          <Skeleton className="h-10 rounded-lg" />
        </div>
        <Skeleton className="h-10 w-full rounded-lg" />
        <Skeleton className="h-10 w-full rounded-lg" />
        <Skeleton className="h-20 w-full rounded-lg" />
        <Skeleton className="h-12 w-full rounded-xl" />
      </div>
    </div>
  );
}

/** Public booking success page skeleton */
export function SkeletonBookSuccess() {
  return (
    <div className="animate-fade-in flex flex-col items-center gap-5 py-16" role="status" aria-label={t.common.loading}>
      <Skeleton className="size-20 rounded-full sm:size-24" />
      <Skeleton className="h-7 w-52 rounded-lg" />
      <Skeleton className="h-4 w-36 rounded-lg" />
      {/* Matches `BookingSummary`'s own surface: flat below `sm`, boxed from
          `sm:` up (odd/tasks/app-dark-contrast.md T2 follow-up). */}
      <div className="sm:border-border-subtle sm:bg-bg-subtle w-full max-w-sm sm:rounded-2xl sm:border sm:p-5">
        <div className="space-y-2">
          {Array.from({ length: 4 }, (_, i) => (
            <div key={i} className="flex justify-between">
              <Skeleton className="h-4 w-16 rounded-lg" />
              <Skeleton className="h-4 w-28 rounded-lg" />
            </div>
          ))}
        </div>
      </div>
      <div className="flex gap-3">
        <Skeleton className="h-10 w-32 rounded-xl" />
        <Skeleton className="h-10 w-32 rounded-xl" />
      </div>
    </div>
  );
}
