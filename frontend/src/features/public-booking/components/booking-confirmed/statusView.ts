import type { BookingStatus, CollectionStatus } from '@/shared/types/api.types';

/**
 * Every screen `BookingConfirmed` can show, as one value instead of the
 * seven independent booleans (`isLoading`, `timedOut`, `paymentUnderReview`,
 * `linkExpired`, `linkNotFound`, plus `status`/`collectionStatus` read as
 * flags) it used to switch on. That shape could represent combinations that
 * can never actually happen (`linkExpired && timedOut`, say) and had no slot
 * at all for "the request itself failed" — a network drop or a 500 read the
 * same as "still processing" until the 120s timeout copy took over, which
 * told the truth about neither.
 */
export type BookingStatusView =
  | { kind: 'loading' }
  | { kind: 'link_expired' }
  | { kind: 'link_not_found' }
  | { kind: 'error' }
  | { kind: 'under_review' }
  | { kind: 'timed_out' }
  | { kind: 'pending' }
  | { kind: 'cancelled' }
  | { kind: 'confirmed' };

export interface BookingStatusSignals {
  status: BookingStatus | undefined;
  collectionStatus: CollectionStatus | undefined;
  isLoading: boolean;
  /** True when the `GET /book/status` query is in its error state — see `useBookingStatus`. */
  isError: boolean;
  timedOut: boolean;
  /** MercadoPago's own back_url carries `?status=pending` — see `BookSuccessPage`. */
  paymentUnderReview: boolean;
  /** `resolveLink` (`internal/bookings/public.go`) answered 410. */
  linkExpired: boolean;
  /** `resolveLink` answered 404. */
  linkNotFound: boolean;
}

/**
 * Resolves the independent signals polling `GET /book/status` produces into
 * one state `BookingConfirmed` can `switch` over exhaustively.
 *
 * Order encodes the same priority the old `if` chain did: a dead link is
 * permanent and wins over everything; MercadoPago's own "under review" signal
 * outranks our polling failing since it is more specific than "we don't
 * know"; a genuine fetch failure (no data at all) is shown before the 120s
 * timeout copy, which otherwise misleads with "puede demorar" language a
 * dropped connection will never resolve on its own; and every one of these
 * steps aside once the booking reaches a terminal status.
 */
export function resolveBookingStatusView(signals: BookingStatusSignals): BookingStatusView {
  const { status, collectionStatus, isLoading, isError, timedOut, paymentUnderReview, linkExpired, linkNotFound } =
    signals;
  const isTerminal = status === 'confirmed' || status === 'cancelled' || status === 'completed';
  const hasData = !!status || !!collectionStatus;

  if (linkExpired) return { kind: 'link_expired' };
  if (linkNotFound) return { kind: 'link_not_found' };
  if (paymentUnderReview && !isTerminal) return { kind: 'under_review' };
  if (isError && !hasData) return { kind: 'error' };
  if (timedOut && !isTerminal) return { kind: 'timed_out' };
  if (isLoading || !hasData) return { kind: 'loading' };
  if (status === 'pending' && collectionStatus === 'unpaid') return { kind: 'pending' };
  if (status === 'cancelled') return { kind: 'cancelled' };
  return { kind: 'confirmed' };
}
