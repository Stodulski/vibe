import { SkeletonSlotGrid } from '../../components/SkeletonSlotGrid';
import { BOOKING_MESSAGES } from '../../messages';
import type { DurationMinutes } from '../../api/types';
import { FullDayPanel } from './FullDayPanel';
import type { EmptyDay } from './emptyDay';

const t = BOOKING_MESSAGES;

/** Who the full-day panel belongs to, and where its next-day and WhatsApp actions lead. */
export interface FullDayContext {
  slug: string;
  phone: string;
  /** The chosen duration, or `null` while the visitor has not answered that question. */
  duration: DurationMinutes | null;
  /** Selects another day, from the panel's "Ver el ..." action. */
  onSelectDate: (date: Date) => void;
}

interface EmptyDayStateProps extends FullDayContext {
  emptyDay: EmptyDay;
  /** The selected day, `YYYY-MM-DD`. */
  dateStr: string;
}

/**
 * What an open day shows in place of its courts when none has a free slot to
 * offer: a skeleton while the other durations are checked, the full-day panel
 * when the day is booked out, or a hint when a filter is what hides the slots.
 */
export function EmptyDayState({ emptyDay, dateStr, slug, phone, duration, onSelectDate }: EmptyDayStateProps) {
  if (emptyDay === 'checking') {
    return (
      <div className="animate-fade-in">
        <SkeletonSlotGrid />
      </div>
    );
  }

  if (emptyDay === 'full') {
    return (
      <div className="animate-fade-in">
        <FullDayPanel slug={slug} date={dateStr} duration={duration} phone={phone} onSelectDate={onSelectDate} />
      </div>
    );
  }

  return (
    <div className="text-text-tertiary animate-fade-in space-y-2 py-8 text-center text-sm">
      <p>{t.publicBooking.noAvailability}</p>
      <p>{t.publicBooking.filterHint}</p>
    </div>
  );
}
