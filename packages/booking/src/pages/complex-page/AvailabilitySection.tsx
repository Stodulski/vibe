import type { ReactNode } from 'react';
import { SkeletonSlotGrid } from '../../components/SkeletonSlotGrid';
import { CourtSelector, type SelectedSlot } from '../../components/CourtSelector';
import { BOOKING_MESSAGES } from '../../messages';
import type { CourtAvailability } from '../../api/types';
import { AvailabilityErrorState } from './AvailabilityErrorState';
import { EmptyDayState, type FullDayContext } from './EmptyDayState';
import type { EmptyDay } from './emptyDay';
import { WaitlistTakenNotice } from './WaitlistTakenNotice';

const t = BOOKING_MESSAGES;

interface AvailabilitySectionProps {
  isLoading: boolean;
  /** True when `useAvailability` failed — a network drop or a 500. Checked
   * before `isOpen` so a failed request never renders as "Cerrado". */
  isError: boolean;
  onRetry: () => void;
  /** A previous answer is on screen while a new one loads — see `useAvailability`. */
  isStale: boolean;
  isOpen: boolean;
  dateStr: string;
  courts: CourtAvailability[];
  selectedSlot: SelectedSlot | null;
  onSelect: (slot: SelectedSlot) => void;
  onContinue: (selection?: SelectedSlot) => void;
  pendingStartTime: string | null;
  onPendingStartTimeChange: (startTime: string | null) => void;
  /** Why the grid has nothing to offer, or `null` when it has courts to show. See `useEmptyDay`. */
  emptyDay: EmptyDay | null;
  /** The complex, the duration and the day selection, for the full-day panel. */
  fullDay: FullDayContext;
  /**
   * The visitor arrived from a waitlist email and someone took that place
   * first. See `waitlistNoticeApplies`.
   */
  waitlistTakenFirst?: boolean | undefined;
}

/** Puts the waitlist notice above what is still on offer, when it applies. */
function withTakenNotice(show: boolean, node: ReactNode): ReactNode {
  if (!show) return node;
  return (
    <>
      <WaitlistTakenNotice />
      {node}
    </>
  );
}

export function AvailabilitySection({
  isLoading,
  isError,
  onRetry,
  isStale,
  isOpen,
  dateStr,
  courts,
  selectedSlot,
  onSelect,
  onContinue,
  pendingStartTime,
  onPendingStartTimeChange,
  emptyDay,
  fullDay,
  waitlistTakenFirst = false,
}: AvailabilitySectionProps) {
  // A change of date or duration refetches. The previous hours stay in the
  // query as placeholder data, but showing them (even dimmed) read as a page
  // that did not react: the skeleton says the new hours are on their way.
  if (isLoading || isStale) {
    return (
      <div key="skeleton" className="animate-fade-in">
        <SkeletonSlotGrid />
      </div>
    );
  }

  // Checked before `isOpen`: with no data (a failed fetch, even after
  // retries) `isOpen` would otherwise fall through to "closed", telling a
  // player the club doesn't open that day when the real problem is the
  // request itself.
  if (isError) {
    return <AvailabilityErrorState onRetry={onRetry} />;
  }


  if (isOpen && emptyDay) {
    return withTakenNotice(waitlistTakenFirst, <EmptyDayState emptyDay={emptyDay} dateStr={dateStr} {...fullDay} />);
  }

  if (isOpen) {
    return (
      <div
        key={`courts-${dateStr}`}
        className="animate-fade-in"
      >
        {waitlistTakenFirst && <WaitlistTakenNotice />}
        <CourtSelector
          courts={courts}
          slug={fullDay.slug}
          date={dateStr}
          selected={selectedSlot}
          onSelect={onSelect}
          onContinue={onContinue}
          pendingStartTime={pendingStartTime}
          onPendingStartTimeChange={onPendingStartTimeChange}
        />
      </div>
    );
  }

  return (
    <p key="closed" className="text-text-tertiary animate-fade-in py-8 text-center text-sm">
      {t.publicBooking.closed}
    </p>
  );
}
