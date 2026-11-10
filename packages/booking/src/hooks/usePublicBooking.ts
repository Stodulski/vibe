import { HTTPError } from 'ky';
import { toast } from 'sonner';
import { usePublicBookingApi } from '../api/context';
import { BOOKING_MESSAGES } from '../messages';
import { getHttpErrorMessage } from '../lib/httpErrors';
import { getProblem } from '../lib/ApiError';
import { conflictCopy, HOLD_EXPIRED_KIND, turnstileCopy } from '../lib/bookingRefusals';
import { useIdempotentMutation, type WithAttemptKey } from '@vibe/ui';
import type { PublicBookingRequest } from '../api/types';

const t = BOOKING_MESSAGES;

interface UsePublicBookingOptions {
  /**
   * The chosen hours are gone — `slot-unavailable` alone, not any 409. The
   * caller sends the visitor back to slot selection, which is the right move
   * only when picking another hour can actually succeed.
   */
  onConflict?: () => void;
  /**
   * 503: the booking was created but MercadoPago's preference could not be
   * (`internal/bookings/public.go`), so the server cancelled it and freed the
   * slot. A toast that self-dismisses is the wrong shape for this one — the
   * caller renders a persistent inline error instead, so this fires in place
   * of the generic toast rather than alongside it.
   */
  onPaymentLinkError?: () => void;
  /**
   * 409 `hold-expired`: the hold this booking converts is gone (expired,
   * released, or already used). The caller sends the visitor back to the
   * hours with the expired notice, in place of a toast.
   */
  onHoldExpired?: () => void;
}

export function usePublicBooking(options?: UsePublicBookingOptions) {
  const api = usePublicBookingApi();
  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<PublicBookingRequest>) => api.createBooking(data, attemptKey),
    onError: (error) => {
      if (error instanceof HTTPError) {
        if (error.response.status === 409) {
          const kind = getProblem(error)?.kind;
          if (kind === HOLD_EXPIRED_KIND && options?.onHoldExpired) {
            options.onHoldExpired();
            return;
          }
          toast.error(conflictCopy(kind));
          // Only a genuinely taken slot earns the trip back to the picker.
          // A duplicate would hit the same refusal on the second attempt, and
          // a stale-version/generic conflict is not about the slot at all.
          if (kind === 'slot-unavailable') options?.onConflict?.();
          return;
        }
        if (error.response.status === 403) {
          toast.error(t.publicBooking.accountBlocked);
          return;
        }
        if (error.response.status === 422) {
          const copy = turnstileCopy(error);
          if (copy) {
            toast.error(copy);
            return;
          }
        }
        if (error.response.status === 503) {
          options?.onPaymentLinkError?.();
          return;
        }
        toast.error(getHttpErrorMessage(error, t.publicBooking.bookingCreateError));
      } else {
        toast.error(t.publicBooking.bookingCreateError);
      }
    },
  });
}
