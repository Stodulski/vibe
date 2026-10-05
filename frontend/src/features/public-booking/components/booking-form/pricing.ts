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
 * The flat service fee, in centavos: 1000 ARS per online payment.
 *
 * Restates backend/internal/pricing/pricing.go.
 * Changing it alone fails CI: .github/scripts/check-service-fee.mjs compares this against the other copies.
 */
const SERVICE_FEE_FALLBACK_CENTAVOS = 100_000;

/**
 * Computes the online-payment breakdown for a booking slot.
 * Prefers the backend-provided `serviceFee` (via `slotInfo.serviceFee`) so
 * that frontend and backend never diverge silently. Falls back to the flat
 * fee only when the backend value is not yet available (no quote endpoint
 * exists at this stage).
 */
export function computeBookingPricing(slotInfo: BookingSlotInfo): BookingPricing {
  const depositAmount = Math.round((slotInfo.price * slotInfo.depositPercentage) / 100);
  const hasDeposit = depositAmount > 0;
  const mpAmount = hasDeposit ? depositAmount : slotInfo.price;
  const serviceFee = slotInfo.serviceFee ?? SERVICE_FEE_FALLBACK_CENTAVOS;
  const totalOnline = mpAmount + serviceFee;
  const remainingAmount = slotInfo.price - mpAmount;

  return { depositAmount, hasDeposit, mpAmount, serviceFee, totalOnline, remainingAmount };
}
