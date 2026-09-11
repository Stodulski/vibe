import { ComplexHeader, DateSelector, type SelectedSlot } from '@/features/public-booking';
import type {
  AvailabilityData,
  CourtAvailability,
  DurationMinutes,
  PublicComplex,
  Schedule,
  Sport,
} from '@/shared/types/api.types';
import { PhoneBookingPanel } from './PhoneBookingPanel';
import { AvailabilitySection } from './AvailabilitySection';
import { BookingSteps } from './booking-steps/BookingSteps';

interface ComplexPageContentProps {
  complex: PublicComplex;
  schedules: Schedule[];
  mpConnected: boolean;
  selectedDate: Date;
  onDateSelect: (date: Date) => void;
  availableSports: Sport[];
  sportFilter: Sport | null;
  onSportFilterChange: (sport: Sport | null) => void;
  availLoading: boolean;
  /** True when `useAvailability` failed — a network drop or a 500, not "closed". */
  availError: boolean;
  onAvailRetry: () => void;
  /** True while a previous answer is still on screen and a new one is in flight. */
  availStale: boolean;
  availability: AvailabilityData | undefined;
  dateStr: string;
  filteredCourts: CourtAvailability[];
  selectedSlot: SelectedSlot | null;
  onSelectSlot: (slot: SelectedSlot) => void;
  /** With a selection when the tap that chose it also continues. */
  onContinue: (selection?: SelectedSlot) => void;
  duration: DurationMinutes;
  /**
   * `null` until the visitor actually answers the duration question — unlike
   * `duration`, which is defaulted for the availability fetch. This is what
   * the duration `StepChoice` uses for its `selected` prop, so no option
   * renders as chosen before anything has been.
   */
  selectedDuration: DurationMinutes | null;
  onDurationChange: (duration: DurationMinutes) => void;
  /** The hour whose court question is open; lives in the URL with the rest. */
  pendingStartTime: string | null;
  onPendingStartTimeChange: (startTime: string | null) => void;
  /** Questions the URL already answered, so the steps open past them. */
  answeredFromUrl: { sport: boolean; duration: boolean };
}

/**
 * The public layout centres every one of its pages in `max-w-2xl` — 672px,
 * the right width for the confirm form and half the screen wasted on a grid
 * of times. Rather than change that cap for pages that need it narrow, this
 * one widens past it, and only where the room demonstrably exists: fixed
 * negative margins at each breakpoint, never a `vw` calculation that a
 * scrollbar can turn into horizontal scroll.
 */
const CONTENT_WIDTH = 'min-w-0 flex-1 space-y-6 animate-fade-in sm:space-y-10 lg:-mx-24 xl:-mx-40 2xl:-mx-64';

/**
 * The storefront: what a visitor sees before they have chosen anything.
 *
 * No step indicator. It used to open with one, fixed at step 1 of 3, which
 * announced that booking here takes three steps to someone who had not yet
 * decided to take the first. Progress belongs inside the form it measures —
 * confirm and pay still carry it.
 */
export function ComplexPageContent({
  complex,
  schedules,
  mpConnected,
  selectedDate,
  onDateSelect,
  ...flow
}: ComplexPageContentProps) {
  // A complex that cannot be booked online is not shown the machinery for
  // booking online. It used to get all of it — date picker, sport filter and
  // every slot of every court, each one disabled — under a warning banner.
  // Nothing on that page could be acted on; the phone is the one thing that
  // can, so it is the only thing offered.
  if (!mpConnected) {
    return (
      <div className={CONTENT_WIDTH}>
        <ComplexHeader complex={complex} schedules={schedules} selectedDate={selectedDate} />
        <PhoneBookingPanel phone={complex.phone} />
      </div>
    );
  }

  return (
    <div className={CONTENT_WIDTH}>
      <ComplexHeader complex={complex} schedules={schedules} selectedDate={selectedDate} />

      {/* The date frames everything below it: every question in the flow
          narrows the inventory inside one day, and this is the one that says
          which day. It is also what gets swept across most, so it never hides
          behind a step. */}
      <DateSelector selectedDate={selectedDate} onDateSelect={onDateSelect} schedules={schedules} />

      <BookingFlow {...flow} />
    </div>
  );
}

type BookingFlowProps = Omit<
  ComplexPageContentProps,
  'complex' | 'schedules' | 'mpConnected' | 'selectedDate' | 'onDateSelect'
>;

/** The questions and the hours. */
function BookingFlow({
  availableSports,
  sportFilter,
  onSportFilterChange,
  availLoading,
  availError,
  onAvailRetry,
  availStale,
  availability,
  dateStr,
  filteredCourts,
  selectedSlot,
  onSelectSlot,
  onContinue,
  duration,
  selectedDuration,
  onDurationChange,
  pendingStartTime,
  onPendingStartTimeChange,
  answeredFromUrl,
}: BookingFlowProps) {
  return (
    <BookingSteps
      availableSports={availableSports}
      sportFilter={sportFilter}
      onSportFilterChange={onSportFilterChange}
      duration={duration}
      selectedDuration={selectedDuration}
      onDurationChange={onDurationChange}
      answeredFromUrl={answeredFromUrl}
      timeAnswer={pendingStartTime}
      onEditTime={() => {
        onPendingStartTimeChange(null);
      }}
    >
      <AvailabilitySection
        isLoading={availLoading}
        isError={availError}
        onRetry={onAvailRetry}
        isStale={availStale}
        isOpen={availability?.is_open ?? false}
        dateStr={dateStr}
        courts={filteredCourts}
        selectedSlot={selectedSlot}
        onSelect={onSelectSlot}
        onContinue={onContinue}
        pendingStartTime={pendingStartTime}
        onPendingStartTimeChange={onPendingStartTimeChange}
      />
    </BookingSteps>
  );
}
