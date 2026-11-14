import { AlertTriangle } from 'lucide-react';
import { BookingCalendar } from '@/features/bookings';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingCalendarSkeleton } from '../../components/booking-calendar/BookingCalendarSkeleton';
import { NoBookingsEmptyState } from './booking-content/BookingEmptyStates';
import type { Booking, BlockedSlot, CourtWithPrices, EventBlock, FixedBlock, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface BookingContentProps {
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  filteredBookings: Booking[];
  blockedSlots: BlockedSlot[];
  fixedBlocks?: FixedBlock[] | undefined;
  eventBlocks?: EventBlock[] | undefined;
  isPast: boolean;
  selectedDate: string;
  courts: CourtWithPrices[];
  schedules: Schedule[];
  onSelectBooking: (booking: Booking) => void;
  onCreateFromSlot: (prefill: { court_id: string; date: string; start_time: string }) => void;
  onOpenCreate: () => void;
  onSelectBlockedSlot?: ((slot: BlockedSlot) => void) | undefined;
  onSelectFixedBlock?: ((block: FixedBlock) => void) | undefined;
  onSelectEventBlock?: ((block: EventBlock) => void) | undefined;
}

export function BookingContent({
  isLoading,
  isError,
  onRetry,
  filteredBookings,
  blockedSlots,
  fixedBlocks,
  eventBlocks,
  isPast,
  selectedDate,
  courts,
  schedules,
  onSelectBooking,
  onCreateFromSlot,
  onOpenCreate,
  onSelectBlockedSlot,
  onSelectFixedBlock,
  onSelectEventBlock,
}: BookingContentProps) {
  if (isLoading) return <BookingCalendarSkeleton />;

  if (isError) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.common.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={onRetry}
      />
    );
  }

  const hasNothing =
    filteredBookings.length === 0 && blockedSlots.length === 0 && !fixedBlocks?.length && !eventBlocks?.length;

  // Handed down rather than returned early: an empty day still has courts, and
  // the timeline's whole point is showing their free time — so it keeps
  // rendering and only the list falls back to this message.
  const emptyState = !hasNothing ? null : <NoBookingsEmptyState isPast={isPast} onOpenCreate={onOpenCreate} />;

  return (
    <BookingCalendar
      bookings={filteredBookings}
      blockedSlots={blockedSlots}
      fixedBlocks={fixedBlocks}
      eventBlocks={eventBlocks}
      courts={courts}
      date={selectedDate}
      schedules={schedules}
      emptyState={emptyState}
      onSelectBooking={onSelectBooking}
      onCreateBooking={onCreateFromSlot}
      onSelectBlockedSlot={onSelectBlockedSlot}
      onSelectFixedBlock={onSelectFixedBlock}
      onSelectEventBlock={onSelectEventBlock}
    />
  );
}
