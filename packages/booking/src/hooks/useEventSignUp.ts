import { useMutation, useQueryClient } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../api/context';
import { BOOKING_MESSAGES } from '../messages';
import { signUpRefusalCopy } from '../lib/eventRefusals';
import { storeEventRegistrationToken } from '../lib/eventRegistrationStorage';
import { navigateTo } from '../lib/navigation';
import { queryKeys } from '../lib/queryKeys';
import type { EventSignUpFormData } from '../schemas/event-signup.schema';

/**
 * Holds a spot and hands the person to the Mercado Pago checkout. `onMessage`
 * receives the text to show when the sign-up fails.
 */
export function useEventSignUp(slug: string, eventId: string, onMessage: (message: string) => void) {
  const api = usePublicBookingApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: EventSignUpFormData) =>
      api.signUpForEvent(slug, eventId, {
        first_name: data.first_name,
        last_name: data.last_name,
        phone: data.phone,
        ...(data.email ? { email: data.email } : {}),
      }),
    onSuccess: (result) => {
      if (!result.mp_init_point) {
        onMessage(BOOKING_MESSAGES.publicBooking.paymentLinkError);
        return;
      }
      storeEventRegistrationToken(eventId, result.token);
      navigateTo(result.mp_init_point);
    },
    onError: (error) => {
      onMessage(signUpRefusalCopy(error));
      // A full or closed event changes what the page offers: read it again.
      if (error instanceof HTTPError && error.response.status === 409) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.events.byId(slug, eventId) });
      }
    },
  });
}
