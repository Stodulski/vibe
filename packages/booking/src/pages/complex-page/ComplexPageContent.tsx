import { ComplexHeader } from '../../components/ComplexHeader';
import { DateSelector } from '../../components/DateSelector';
import type { SelectedSlot } from '../../components/CourtSelector';
import type {
  AvailabilityData,
  CourtAvailability,
  CourtWithPrices,
  DurationMinutes,
  PublicComplex,
  Schedule,
  Sport,
} from '../../api/types';
import { CourtDirectory } from './CourtDirectory';
import { PhoneBookingPanel } from './PhoneBookingPanel';
import { AvailabilitySection } from './AvailabilitySection';
import { BookingSteps } from './booking-steps/BookingSteps';
import type { EmptyDay } from './emptyDay';

interface ComplexPageContentProps {
  complex: PublicComplex;
  schedules: Schedule[];
  /** Every active court of the complex. Only the phone-only page lists them. */
  courts: CourtWithPrices[];
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
  /** Why the grid of the selected day is empty, or `null` when it has courts to show. See `useEmptyDay`. */
  emptyDay: EmptyDay | null;
}

/**
 * The public layout centres every one of its pages in `max-w-2xl` — 672px,
 * the right width for the confirm form and half the screen wasted on a grid
 * of times. Rather than change that cap for pages that need it narrow, this
 * one widens past it, and only where the room demonstrably exists: fixed
 * negative margins at each breakpoint, never a `vw` calculation that a
 * scrollbar can turn into horizontal scroll.
 */
const CONTENT_WIDTH = 'flex min-w-0 flex-1 flex-col animate-fade-in lg:-mx-24 xl:-mx-40 2xl:-mx-64';

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
  courts,
  mpConnected,
  selectedDate,
  onDateSelect,
  ...flow
}: ComplexPageContentProps) {
  // A complex that cannot be booked online is not shown the machinery for
  // booking online. It used to get all of it — date picker, sport filter and
  // every slot of every court, each one disabled — under a warning banner.
  // Nothing on that page could be acted on, so the phone is the one thing
  // offered to act on; the courts are listed beneath it, read-only.
  if (!mpConnected) {
    return (
      <div className={CONTENT_WIDTH}>
        <ComplexHeader complex={complex} schedules={schedules} courts={courts} selectedDate={selectedDate} />
        <div className="mt-6 sm:mt-10">
          <PhoneBookingPanel phone={complex.phone} />
        </div>
        <div className="mt-6 max-lg:order-2 sm:mt-10">
          <CourtDirectory courts={courts} />
        </div>
      </div>
    );
  }

  return (
    <div className={CONTENT_WIDTH}>
      <ComplexHeader complex={complex} schedules={schedules} courts={courts} selectedDate={selectedDate} />

      {/* The date frames everything below it: every question in the flow
          narrows the inventory inside one day, and this is the one that says
          which day. It is also what gets swept across most, so it never hides
          behind a step. */}
      <div className="mt-6 sm:mt-10">
        <DateSelector selectedDate={selectedDate} onDateSelect={onDateSelect} schedules={schedules} />
      </div>

      <div className="mt-6 sm:mt-10">
        <BookingFlow {...flow} complex={complex} onDateSelect={onDateSelect} />
      </div>
    </div>
  );
}

type BookingFlowProps = Omit<ComplexPageContentProps, 'schedules' | 'courts' | 'mpConnected' | 'selectedDate'>;

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
  emptyDay,
  complex,
  onDateSelect,
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
        emptyDay={emptyDay}
        fullDay={{ slug: complex.slug, phone: complex.phone, duration: selectedDuration, onSelectDate: onDateSelect }}
      />
    </BookingSteps>
  );
}
