import type { AvailabilitySlot } from '@/shared/types/api.types';

export interface SelectedSlot {
  courtId: string;
  courtName: string;
  sport: string;
  courtType: string;
  /** The owner's description of the court, when there is one. */
  courtDescription?: string | undefined;
  slot: AvailabilitySlot;
  durationMinutes: number;
  totalPrice: number;
  endTime: string;
}
