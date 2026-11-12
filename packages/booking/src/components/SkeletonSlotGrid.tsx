import { Skeleton } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../messages';
import { TIME_GROUPS } from './court-selector/constants';
import type { TimeGroup } from './court-selector/slotMath';

const t = BOOKING_MESSAGES;

// A typical day at `lg:` (8 columns): one row of morning hours, two each for
// the afternoon and the evening, so the page does not jump when they arrive.
const TILES_PER_GROUP: Record<TimeGroup, number> = { morning: 8, afternoon: 16, evening: 16 };

/**
 * Mirrors `CourtSelector`'s hour-major grid, not the court-major layout it
 * replaced (odd/tasks/public-pending-polish.md T1): a period header — the
 * same icon and label `TimeGroupGrid` renders, since neither depends on
 * data — followed by a grid of hour-sized placeholders at the same column
 * breakpoints as `TimeSlotButton` (`grid-cols-3 sm:grid-cols-4 md:grid-cols-6
 * lg:grid-cols-8`). All three parts of the day are drawn, as `TimeGroupList`
 * draws them for a full day, so the page does not grow when hours arrive.
 * Each tile is `h-11`, the nearest step to a real hour button, whose height
 * comes from its content: `py-3` around one `text-sm` line and its border
 * (46px in total).
 *
 * No fake breadcrumb or duration-chip row: those live in `BookingSteps`,
 * outside `AvailabilitySection`, and are already on screen — real, not
 * skeleton — by the time this ever renders there. Depicting them here would
 * only draw a second, fake copy right below the real one.
 */
export function SkeletonSlotGrid() {
  return (
    <div className="animate-fade-in space-y-6" role="status" aria-label={t.publicBooking.checkingAvailability}>
      {TIME_GROUPS.map(({ key, icon: Icon }) => (
        <div key={key}>
          <div className="mb-2 flex items-center gap-1.5">
            <Icon className="text-text-tertiary size-3.5" />
            <Skeleton className="h-3 w-14 rounded-full" />
          </div>
          <div className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8">
            {Array.from({ length: TILES_PER_GROUP[key] }, (_, slotIdx) => (
              <Skeleton key={slotIdx} className="h-11 rounded-xl" />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
