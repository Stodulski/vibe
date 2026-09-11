import type { BookingSlotInfo } from './types';

export interface BookingPricing {
  depositAmount: number;
  hasDeposit: boolean;
  mpAmount: number;
  serviceFee: number;
  totalOnline: number;
  remainingAmount: number;
}

/**
 * Computes the online-payment breakdown for a booking slot.
 * Prefers the backend-provided `serviceFee` (via `slotInfo.serviceFee`) so
 * that frontend and backend never diverge silently. Falls back to the local
 * estimate (7% of the online amount, floored at 100000 centavos) only when
 * the backend value is not yet available (no quote endpoint exists at this
 * stage).
 */
export function computeBookingPricing(slotInfo: BookingSlotInfo): BookingPricing {
  const depositAmount = Math.round((slotInfo.price * slotInfo.depositPercentage) / 100);
  const hasDeposit = depositAmount > 0;
  const mpAmount = hasDeposit ? depositAmount : slotInfo.price;
  const serviceFee = slotInfo.serviceFee ?? Math.max(Math.round((mpAmount * 7) / 100), 100_000);
  const totalOnline = mpAmount + serviceFee;
  const remainingAmount = slotInfo.price - mpAmount;

  return { depositAmount, hasDeposit, mpAmount, serviceFee, totalOnline, remainingAmount };
}
