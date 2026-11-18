import type { AvailabilitySlot } from '../../api/types';

export interface SelectedSlot {
  courtId: string;
  courtName: string;
  sport: string;
  courtType: string;
  slot: AvailabilitySlot;
  durationMinutes: number;
  totalPrice: number;
  endTime: string;
}
