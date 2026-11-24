import type { AvailabilitySlot } from '../../api/types';

export interface SelectedSlot {
  courtId: string;
  courtName: string;
  sport: string;
  courtType: string;
  /** Absent when the owner has not said; with the lighting, what the checkout ticket shows of the court. */
  surface?: string;
  hasLighting: boolean;
  slot: AvailabilitySlot;
  durationMinutes: number;
  totalPrice: number;
  endTime: string;
}
