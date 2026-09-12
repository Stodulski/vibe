import type { Booking, BlockedSlot, CourtWithPrices } from '@/shared/types/api.types';

export interface TimelineColumnData {
  courtId: string;
  courtName: string;
  bookings: Booking[];
  blocked: BlockedSlot[];
}

/**
 * One column per active court, in their existing order. A booking or blocked
 * slot referencing a court that isn't in `activeCourts` anymore (deactivated
 * after the booking was made) gets its own extra column appended at the end
 * instead of silently disappearing — labelled from the item's own
 * `court_name` since the court record is gone.
 */
export function useTimelineColumns({
  activeCourts,
  bookings,
  blockedSlots,
}: {
  activeCourts: CourtWithPrices[];
  bookings: Booking[];
  blockedSlots: BlockedSlot[];
}): TimelineColumnData[] {
  const columns: TimelineColumnData[] = activeCourts.map((court) => ({
    courtId: court.id,
    courtName: court.name,
    bookings: bookings.filter((b) => b.court_id === court.id),
    blocked: blockedSlots.filter((s) => s.court_id === court.id),
  }));

  const knownIds = new Set(activeCourts.map((c) => c.id));
  const orphanIds = [...new Set([...bookings, ...blockedSlots].map((item) => item.court_id))].filter(
    (id) => !knownIds.has(id),
  );

  for (const courtId of orphanIds) {
    const orphanBooking = bookings.find((b) => b.court_id === courtId);
    const orphanBlocked = blockedSlots.find((s) => s.court_id === courtId);
    columns.push({
      courtId,
      courtName: orphanBooking?.court_name ?? orphanBlocked?.court_name ?? courtId,
      bookings: bookings.filter((b) => b.court_id === courtId),
      blocked: blockedSlots.filter((s) => s.court_id === courtId),
    });
  }

  return columns;
}
