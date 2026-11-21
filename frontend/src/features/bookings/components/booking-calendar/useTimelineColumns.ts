import { useMemo } from 'react';
import type { Booking, BlockedSlot, CourtWithPrices, CourtEventBlock, FixedBlock } from '@/shared/types/api.types';
import type { OnlineHold } from '../../schemas/onlineHold.schema';

export interface TimelineColumnData {
  courtId: string;
  courtName: string;
  bookings: Booking[];
  blocked: BlockedSlot[];
  fixed: FixedBlock[];
  event: CourtEventBlock[];
  /** Hours a visitor is booking online on this court. */
  online: OnlineHold[];
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
  fixedBlocks,
  eventBlocks,
  onlineHolds,
}: {
  activeCourts: CourtWithPrices[];
  bookings: Booking[];
  blockedSlots: BlockedSlot[];
  fixedBlocks: FixedBlock[];
  eventBlocks: CourtEventBlock[];
  onlineHolds: OnlineHold[];
}): TimelineColumnData[] {
  return useMemo(() => {
    const columns: TimelineColumnData[] = activeCourts.map((court) => ({
      courtId: court.id,
      courtName: court.name,
      bookings: bookings.filter((b) => b.court_id === court.id),
      blocked: blockedSlots.filter((s) => s.court_id === court.id),
      fixed: fixedBlocks.filter((f) => f.court_id === court.id),
      event: eventBlocks.filter((e) => e.court_id === court.id),
      online: onlineHolds.filter((h) => h.court_id === court.id),
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
        fixed: fixedBlocks.filter((f) => f.court_id === courtId),
        event: eventBlocks.filter((e) => e.court_id === courtId),
        online: onlineHolds.filter((h) => h.court_id === courtId),
      });
    }

    return columns;
  }, [activeCourts, bookings, blockedSlots, fixedBlocks, eventBlocks, onlineHolds]);
}
