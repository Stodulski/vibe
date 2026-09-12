import { findTotalPrice } from './pricing';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

/**
 * The price preview for the booking being filled in, and the deposit that
 * follows from it.
 *
 * The initial duration default (from `prefill`, or a generic 90) is seeded
 * once by `useBookingReset`. This hook does not touch it: it re-runs on every
 * `courtId` change including the initial prefill-driven one, where overwriting
 * a prefill-provided duration would be wrong (see the precedence rule in
 * `useBookingReset`). Picking a court no longer re-defaults the duration
 * either — every court sells the same three durations, so there is nothing
 * court-specific to prefer.
 *
 * Everything here is derived at render time from its inputs — no effect
 * mirrors it into the form. `defaultDepositPesos` is applied to the
 * `deposit_amount` field by the two places that actually change it (selecting
 * "deposit" as the payment option, and — for a stale default left over from
 * an earlier price — `useCreateBookingForm`'s submit handler); `priceRequired`
 * is read directly by `CreateBookingSteps`/`PriceOrManualPriceField` instead
 * of being mirrored into a hidden schema-only field (see 02-bookings-clients.md M3).
 */
export function useBookingPricing({
  courts,
  courtId,
  date,
  startTime,
  durationMinutes,
  depositPercentage,
  price,
  schedules,
}: {
  courts: CourtWithPrices[];
  courtId: string;
  date: string;
  startTime: string;
  durationMinutes: number;
  depositPercentage: number;
  /** Pesos, from the manual-price field — set only when there's no server-computed estimate. */
  price: number | undefined;
  /** Opening hours: they decide which day's card prices the hour. */
  schedules: Schedule[];
}) {
  const activeCourts = courts.filter((c) => c.is_active);
  const selectedCourt = courts.find((c) => c.id === courtId);

  const estimatedPrice = findTotalPrice(courts, courtId, date, startTime, durationMinutes, schedules);

  // `estimatedPrice` is null both for "nothing chosen yet" and for "this hour
  // has no rate", which are not the same thing. Only once a court and a time
  // are picked does a null mean the second — before that, asking for a price
  // and saying no rate exists "for this time" names a time nobody chose.
  const spanChosen = courtId !== '' && date !== '' && startTime !== '';
  const priceRequired = spanChosen && estimatedPrice === null;

  // Cents, from the manual-price field (pesos) — only meaningful when there's
  // no server-computed estimate for this span.
  const manualPriceCents = typeof price === 'number' && !Number.isNaN(price) ? Math.round(price * 100) : null;

  // The price the rest of the form (deposit cap, payment summary) should
  // follow: the server-computed estimate when there is one, otherwise
  // whatever the person typed into the manual-price field.
  const effectivePrice = estimatedPrice ?? manualPriceCents;

  const defaultDepositPesos = depositInPesos(effectivePrice, depositPercentage);

  const maxDepositPesos = effectivePrice !== null ? effectivePrice / 100 : null;

  return {
    activeCourts,
    selectedCourt,
    estimatedPrice,
    priceRequired,
    effectivePrice,
    defaultDepositPesos,
    maxDepositPesos,
  };
}

/**
 * The default deposit in pesos, from a price in cents.
 *
 * Null when there is no price to take a percentage of, or when the complex
 * takes no deposit — both mean "do not prefill an amount", which is different
 * from prefilling zero.
 */
function depositInPesos(priceCents: number | null, percentage: number): number | null {
  if (priceCents === null || percentage <= 0) return null;
  return Math.floor((priceCents * percentage) / 100 / 100);
}
