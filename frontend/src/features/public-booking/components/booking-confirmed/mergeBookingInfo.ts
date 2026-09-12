import type { BookingStatusDetails } from '@/shared/types/api.types';
import type { BookingInfo } from './types';

/**
 * The success page's one source of truth for what to show, field by field:
 * the live `GET /book/status` answer wins whenever it has an opinion, and
 * the `BookingInfo` cached in sessionStorage before the MercadoPago
 * redirect (or handed straight through router state, for the no-deposit
 * flow) covers whatever the answer hasn't caught up to — an older server
 * still returning only `status`/`collection_status`/`refund_status`, or the
 * brief window
 * before the first response lands at all.
 *
 * Returns null when neither source has the minimum needed to render a card
 * at all — no cache, and either no API response yet or one carrying none of
 * these fields — so the caller hides the whole details block instead of
 * rendering one full of holes.
 */
export function mergeBookingInfo(
  details: BookingStatusDetails | undefined,
  cached: BookingInfo | null,
): BookingInfo | null {
  const courtName = details?.court_name ?? cached?.courtName;
  const date = details?.date ?? cached?.date;
  const startTime = details?.start_time ?? cached?.startTime;
  const startsAt = details?.starts_at ?? cached?.startsAt;
  // The end is an instant now, and the gate below still refuses without one.
  // Weakening it to "render what we have" would put a card on screen with the
  // hours half-missing, which is worse than the block being hidden: the person
  // reading it has just paid. It was `end_time` until backend dropped the
  // column, an `HH:MM` that read "01:00" for a booking finishing at one in the
  // morning of the next day and said nothing about which day that was.
  const endsAt = details?.ends_at ?? cached?.endsAt;
  const price = details?.price ?? cached?.price;
  const depositAmount = details?.deposit_amount ?? cached?.depositAmount;
  const complexName = details?.complex_name ?? cached?.complexName;

  if (
    !courtName ||
    !date ||
    !startTime ||
    !startsAt ||
    !endsAt ||
    price == null ||
    depositAmount == null ||
    !complexName
  ) {
    return null;
  }

  return {
    courtName,
    date,
    startTime,
    startsAt,
    endsAt,
    price,
    depositAmount,
    complexName,
    complexPhone: details?.complex_phone ?? cached?.complexPhone ?? '',
    cancellationHours: details?.cancellation?.cancellation_hours ?? cached?.cancellationHours ?? 0,
    clientPhone: cached?.clientPhone ?? '',
    complexAddress: details?.complex_address ?? cached?.complexAddress,
    sport: details?.sport ?? cached?.sport,
    courtType: details?.court_type ?? cached?.courtType,
    serviceFee: details?.service_fee ?? cached?.serviceFee,
    remainingAmount: details?.remaining_amount ?? cached?.remainingAmount ?? price - depositAmount,
    cancellation: details?.cancellation
      ? {
          canCancel: details.cancellation.can_cancel,
          refundDeadline: details.cancellation.refund_deadline ?? null,
          canRefundNow: details.cancellation.can_refund_now,
        }
      : undefined,
  };
}
