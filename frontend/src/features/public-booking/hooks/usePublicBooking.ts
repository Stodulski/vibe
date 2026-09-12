import { HTTPError } from 'ky';
import { toast } from 'sonner';
import { publicBookingApi } from '../api/public-booking.api';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { PublicBookingRequest } from '@/shared/types/api.types';

const t = ES_AR;

interface UsePublicBookingOptions {
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

export function usePublicBooking(options?: UsePublicBookingOptions) {
  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<PublicBookingRequest>) =>
      publicBookingApi.createBooking(data, attemptKey),
    onError: (error) => {
      if (error instanceof HTTPError) {
        if (error.response.status === 409) {
          toast.error(t.publicBooking.slotConflict);
          options?.onConflict?.();
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
