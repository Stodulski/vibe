import { SkeletonSlotGrid, CourtSelector, type SelectedSlot } from '@/features/public-booking';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtAvailability } from '@/shared/types/api.types';
import { AvailabilityErrorState } from './AvailabilityErrorState';

const t = ES_AR;

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
}: AvailabilitySectionProps) {
  if (isLoading) {
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

  if (isOpen) {
    return (
      <div
        key={`courts-${dateStr}`}
        // Dimmed rather than replaced by a skeleton: the previous times are
        // still true until the new ones land, and someone comparing durations
        // needs them to stay put. `aria-busy` says the same thing to a screen
        // reader that the opacity says to an eye.
        className={cn('animate-fade-in transition-opacity', isStale && 'pointer-events-none opacity-50')}
        aria-busy={isStale}
      >
        <CourtSelector
          courts={courts}
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
