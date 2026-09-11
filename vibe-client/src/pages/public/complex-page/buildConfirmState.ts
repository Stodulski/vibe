import type { SelectedSlot, BookingSlotInfo } from '@/features/public-booking';
import type { PublicComplex } from '@/shared/types/api.types';

export function buildConfirmState(
  complex: PublicComplex,
  selectedSlot: SelectedSlot,
  dateStr: string,
): BookingSlotInfo {
  return {
    complexId: complex.id,
    complexName: complex.name,
    complexPhone: complex.phone,
    courtId: selectedSlot.courtId,
    courtName: selectedSlot.courtName,
    sport: selectedSlot.sport,
    courtType: selectedSlot.courtType,
    courtDescription: selectedSlot.courtDescription,
    date: dateStr,
    startTime: selectedSlot.slot.start_time,
    endTime: selectedSlot.endTime,
    durationMinutes: selectedSlot.durationMinutes,
    price: selectedSlot.totalPrice,
    depositPercentage: complex.deposit_percentage,
    cancellationHours: complex.cancellation_hours,
  };
}
