import { Skeleton } from '../../ui/skeleton';
import { messages } from '../../../messages';

/** Cancel booking info skeleton */
export function SkeletonCancelInfo() {
  return (
    <div
      className="animate-fade-in mx-auto flex w-full max-w-md flex-col items-center gap-4 px-4 py-16"
      role="status"
      aria-label={messages.loading}
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

/**
 * The summary card placeholder in `SkeletonBookConfirm`: `BookingSummaryCard`'s
 * padding and radius, drawn as a neutral block because the real card is solid
 * green. Its blocks stand in for the venue, the day and hours, the court and
 * its chips, the tear line, the money rows, and the cancellation line, summing
 * to the real card's height (about 477px at `sm:` with no deposit).
 */
function SkeletonSummaryCard() {
  return (
    <div className="bg-bg-subtle rounded-3xl p-6 sm:p-8">
      {/* Venue, day, hours in large type, then the court and its chips. */}
      <div className="space-y-6">
        <Skeleton className="h-5 w-32 rounded-lg" />
        <div>
          <Skeleton className="h-5 w-44 rounded-lg" />
          <Skeleton className="h-9 w-52 rounded-lg" />
        </div>
        <div>
          <Skeleton className="h-6 w-40 rounded-lg" />
          <div className="mt-2 flex gap-1.5">
            <Skeleton className="h-5.5 w-16 rounded-full" />
            <Skeleton className="h-5.5 w-20 rounded-full" />
          </div>
        </div>
      </div>
      {/* Tear line, as `TicketStub` spaces it. */}
      <div className="bg-border-subtle -mx-6 my-6 h-px sm:-mx-8" />
      <Skeleton className="h-5 w-full rounded-lg" />
      {/* Money rows: the service fee with its note, then the total band. */}
      <div className="mt-3 space-y-3">
        <div>
          <Skeleton className="h-5 w-full rounded-lg" />
          <Skeleton className="mt-1 h-4 w-3/4 rounded-lg" />
        </div>
        <Skeleton className="h-15 w-full rounded-2xl" />
      </div>
      {/* Cancellation window line. */}
      <Skeleton className="mt-6 h-5 w-2/3 rounded-lg" />
    </div>
  );
}

/**
 * Public booking confirm page skeleton, shaped like `BookConfirmPage` in the
 * same order: step indicator, back link, hold countdown, then `BookingForm`'s
 * column of summary card and fields.
 */
export function SkeletonBookConfirm() {
  return (
    <div className="animate-fade-in w-full space-y-6 sm:space-y-8" role="status" aria-label={messages.loading}>
      {/* Step indicator, as `StepIndicator` draws it: 28px circles, a step label
          from `sm:` up, and the "Paso N de 3" line below them on mobile only. */}
      <div className="flex flex-col items-center gap-1.5">
        <div className="flex items-center justify-center">
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="flex items-center">
              {i > 0 && <Skeleton className="h-px w-8 rounded-full sm:w-12" />}
              <div className="flex items-center gap-2">
                <Skeleton className="size-7 rounded-full" />
                <Skeleton className="hidden h-3 w-14 rounded-full sm:block" />
              </div>
            </div>
          ))}
        </div>
        <Skeleton className="h-4 w-44 rounded-full sm:hidden" />
      </div>
      {/* Back link: 48px tall below `sm:` (a touch target), a bare 20px text line from `sm:` up. */}
      <div className="flex min-h-12 items-center sm:min-h-0">
        <Skeleton className="h-5 w-36 rounded-lg" />
      </div>
      {/* Hold countdown: one `text-3xl` line, 36px tall, centered. */}
      <Skeleton className="mx-auto h-9 w-24 rounded-lg" />
      {/* `BookingForm`'s column: the summary card, then the fields. */}
      <div className="space-y-5">
        <SkeletonSummaryCard />
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
    </div>
  );
}

/** Public booking success page skeleton */
export function SkeletonBookSuccess() {
  return (
    <div className="animate-fade-in flex flex-col items-center gap-5 py-16" role="status" aria-label={messages.loading}>
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
