import { Skeleton } from '@/shared/components/ui/skeleton';
import { BOOKING_MESSAGES } from '../messages';
import { TIME_GROUPS } from './court-selector/constants';

const t = BOOKING_MESSAGES;

/**
 * Mirrors `CourtSelector`'s hour-major grid, not the court-major layout it
 * replaced (odd/tasks/public-pending-polish.md T1): a period header — the
 * same icon and label `TimeGroupGrid` renders, since neither depends on
 * data — followed by a grid of hour-sized placeholders at the same column
 * breakpoints as `TimeSlotButton` (`grid-cols-3 sm:grid-cols-4 md:grid-cols-6
 * lg:grid-cols-8`). Two groups, not three: a club open only in the evening
 * still gets a plausible loading shape, and a third group would only add
 * height a one-group club never fills once real data lands.
 *
 * No fake breadcrumb or duration-chip row: those live in `BookingSteps`,
 * outside `AvailabilitySection`, and are already on screen — real, not
 * skeleton — by the time this ever renders there. Depicting them here would
 * only draw a second, fake copy right below the real one.
 */
export function SkeletonSlotGrid() {
  return (
    <div className="animate-fade-in space-y-6" role="status" aria-label={t.publicBooking.checkingAvailability}>
      {TIME_GROUPS.slice(1).map(({ key, icon: Icon }) => (
        <div key={key}>
          <div className="mb-2 flex items-center gap-1.5">
            <Icon className="text-text-tertiary size-3.5" />
            <Skeleton className="h-3 w-14 rounded-full" />
          </div>
          <div className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8">
            {Array.from({ length: 8 }, (_, slotIdx) => (
              <Skeleton key={slotIdx} className="h-14 rounded-xl" />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
