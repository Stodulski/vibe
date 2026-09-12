import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { useSelectedComplex } from '@/features/complex';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useBookingsPage } from './bookings/useBookingsPage';
import { BookingContent } from './bookings/BookingContent';
import { BookingsPageHeader } from './bookings/BookingsPageHeader';
import { DateNavStrip } from './bookings/DateNavStrip';
import { BookingsPageModals } from './bookings/BookingsPageModals';
import { useBlockSlotSection } from './bookings/useBlockSlotSection';

const t = ES_AR;

export default function BookingsPage() {
  usePageTitle(t.bookings.title);
  const { selectedComplexId } = useSelectedComplex();

  // Guarded here so `useBookingsPage`/`useBlockSlotSection` below only ever
  // mount once there's a real complex id — they take `complexId: string`,
  // not `string | null`, so there's no `?? ''` type-level fallback to hide
  // behind.
  if (!selectedComplexId) return null;

  return <BookingsPageContent complexId={selectedComplexId} />;
}

function BookingsPageContent({ complexId }: { complexId: string }) {
  const state = useBookingsPage(complexId);
  const blockSlot = useBlockSlotSection(complexId);

  return (
    <div className="animate-fade-in">
      <BookingsPageHeader
        isPast={state.isPast}
        hasCourts={state.courts.length > 0}
        onBlockSlot={() => {
          blockSlot.setBlockOpen(true);
        }}
        onCreateBooking={state.handleOpenCreate}
      />

      {/* A rule does what the strip's card used to: mark where picking a day
          ends and reading that day begins. Padding above it and margin below
          keep the same air on both sides of the line. */}
      <div className="border-border-subtle mb-6 border-b pb-6 sm:mb-8 sm:pb-8">
        <DateNavStrip
          selectedDate={state.selectedDate}
          dateLabel={state.dateLabel}
          isToday={state.isToday}
          calendarOpen={state.calendarOpen}
          onCalendarOpenChange={state.setCalendarOpen}
          onDateSelect={state.setSelectedDate}
          onPrevDay={state.handlePrevDay}
          onNextDay={state.handleNextDay}
          onGoToToday={state.handleGoToToday}
        />
      </div>

      {/* Content */}
      <div>
        <BookingContent
          isLoading={state.isLoading}
          isError={state.isError}
          onRetry={() => {
            void state.refetch();
          }}
          filteredBookings={state.bookings}
          blockedSlots={state.blockedSlots}
          isPast={state.isPast}
          selectedDate={state.selectedDate}
          courts={state.courts}
          schedules={state.schedules}
          onSelectBooking={state.handleSelectBooking}
          onCreateFromSlot={state.handleCreateFromSlot}
          onOpenCreate={state.handleOpenCreate}
          onSelectBlockedSlot={blockSlot.setSelectedSlot}
        />
      </div>

      <BookingsPageModals state={state} blockSlot={blockSlot} selectedComplexId={complexId} />
    </div>
  );
}
