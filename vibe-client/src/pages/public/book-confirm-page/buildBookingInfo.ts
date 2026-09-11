import type { BookingSlotInfo, BookingInfo } from '@/features/public-booking';
import { MINUTES_PER_DAY, venueInstant } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';

export function buildBookingInfo(slotInfo: BookingSlotInfo, clientPhone: string): BookingInfo {
  const deposit = Math.round((slotInfo.price * slotInfo.depositPercentage) / 100);
  const depositAmount = deposit > 0 ? deposit : slotInfo.price;

  // The instants for the copy cached before payment. The picked slot carries
  // two clock readings, and a slot that runs past midnight comes back with an
  // end smaller than its start — so the end's minutes are pushed onto the next
  // day rather than compared. The server's own starts_at/ends_at replace both
  // the moment GET /book/status answers.
  const startMin = timeToMinutes(slotInfo.startTime);
  const rawEndMin = timeToMinutes(slotInfo.endTime);
  const endMin = rawEndMin > startMin ? rawEndMin : rawEndMin + MINUTES_PER_DAY;

  return {
    courtName: slotInfo.courtName,
    date: slotInfo.date,
    startTime: slotInfo.startTime,
    startsAt: venueInstant(slotInfo.date, startMin) ?? '',
    endsAt: venueInstant(slotInfo.date, endMin) ?? '',
    price: slotInfo.price,
    depositAmount,
    complexName: slotInfo.complexName,
    complexPhone: slotInfo.complexPhone,
    cancellationHours: slotInfo.cancellationHours,
    clientPhone,
  };
}
