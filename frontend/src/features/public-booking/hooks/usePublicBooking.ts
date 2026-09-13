import { HTTPError } from 'ky';
import { toast } from 'sonner';
import { publicBookingApi } from '../api/public-booking.api';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { getProblem } from '@/shared/lib/ApiError';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { PublicBookingRequest } from '@/shared/types/api.types';

const t = ES_AR;

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
}

/**
 * The text a 409 from `POST /book` should show, chosen by the problem+json
 * `kind` the backend minted it with (`internal/httpx/problem.go`).
 *
 * `slot-unavailable` and `duplicate-booking` are both 409s and mean opposite
 * things to the person reading them: one says somebody else took the hours,
 * the other says they already booked them themselves. Switching on the status
 * alone told everyone the first story.
 */
function conflictCopy(kind: string | undefined): string {
  switch (kind) {
    case 'slot-unavailable':
      return t.publicBooking.slotConflict;
    case 'duplicate-booking':
      return t.publicBooking.duplicateBooking;
    // `stale-version`, the generic `conflict`, and any kind a later backend
    // adds: nothing specific can be said, so ask for a retry.
    default:
      return t.publicBooking.bookingConflict;
  }
}

export function usePublicBooking(options?: UsePublicBookingOptions) {
  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<PublicBookingRequest>) =>
      publicBookingApi.createBooking(data, attemptKey),
    onError: (error) => {
      if (error instanceof HTTPError) {
        if (error.response.status === 409) {
          const kind = getProblem(error)?.kind;
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
