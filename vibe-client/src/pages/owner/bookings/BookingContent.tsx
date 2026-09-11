import { AlertTriangle } from 'lucide-react';
import { BookingCalendar } from '@/features/bookings';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingLoadingSkeleton } from './booking-content/BookingLoadingSkeleton';
import { NoBookingsEmptyState } from './booking-content/BookingEmptyStates';
import type { Booking, BlockedSlot, CourtWithPrices, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface BookingContentProps {
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  filteredBookings: Booking[];
  blockedSlots: BlockedSlot[];
  isPast: boolean;
  selectedDate: string;
  courts: CourtWithPrices[];
  schedules: Schedule[];
  onSelectBooking: (booking: Booking) => void;
  onCreateFromSlot: (prefill: { court_id: string; date: string; start_time: string }) => void;
  onOpenCreate: () => void;
  onSelectBlockedSlot?: ((slot: BlockedSlot) => void) | undefined;
}

export function BookingContent({
  isLoading,
  isError,
  onRetry,
  filteredBookings,
  blockedSlots,
  isPast,
  selectedDate,
  courts,
  schedules,
  onSelectBooking,
  onCreateFromSlot,
  onOpenCreate,
  onSelectBlockedSlot,
}: BookingContentProps) {
  if (isLoading) return <BookingLoadingSkeleton />;

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

  const hasNothing = filteredBookings.length === 0 && blockedSlots.length === 0;

  // Handed down rather than returned early: an empty day still has courts, and
  // the timeline's whole point is showing their free time — so it keeps
  // rendering and only the list falls back to this message.
  const emptyState = !hasNothing ? null : <NoBookingsEmptyState isPast={isPast} onOpenCreate={onOpenCreate} />;

  return (
    <BookingCalendar
      bookings={filteredBookings}
      blockedSlots={blockedSlots}
      courts={courts}
      date={selectedDate}
      schedules={schedules}
      emptyState={emptyState}
      onSelectBooking={onSelectBooking}
      onCreateBooking={onCreateFromSlot}
      onSelectBlockedSlot={onSelectBlockedSlot}
    />
  );
}
