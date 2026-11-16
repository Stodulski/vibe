import { useMutation, useQueryClient } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../api/context';
import type { PublicOpenPlayEvent } from '../api/types';
import { BOOKING_MESSAGES } from '../messages';
import { signUpRefusalCopy } from '../lib/eventRefusals';
import { openPlayDeclaresLevel, toDeclared } from '../lib/declaredPlayers';
import { isModality } from '../lib/traitLabels';
import { storeEventRegistrationToken } from '../lib/eventRegistrationStorage';
import { navigateTo } from '../lib/navigation';
import { queryKeys } from '../lib/queryKeys';
import type { EventSignUpFormData } from '../schemas/event-signup.schema';

/**
 * Holds a spot and hands the person to the Mercado Pago checkout. An event with
 * a modality also gets the person's gender and level; one without, the request
 * is the same as it was before the traits. The request carries the Turnstile
 * token when there is one. `onMessage` receives the text to show when the
 * sign-up fails.
 */
export function useEventSignUp(
  slug: string,
  event: Pick<PublicOpenPlayEvent, 'id' | 'sport' | 'modality' | 'level_min' | 'level_max'>,
  onMessage: (message: string) => void,
) {
  const api = usePublicBookingApi();
  const queryClient = useQueryClient();
  const eventId = event.id;
  const declares = isModality(event.modality) || openPlayDeclaresLevel(event);
  return useMutation({
    mutationFn: ({ data, token }: { data: EventSignUpFormData; token?: string | undefined }) =>
      api.signUpForEvent(slug, eventId, {
        first_name: data.first_name,
        last_name: data.last_name,
        phone: data.phone,
        ...(data.email ? { email: data.email } : {}),
        ...(declares ? toDeclared(data.gender, data.level) : {}),
        ...(token ? { turnstile_token: token } : {}),
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
      onMessage(signUpRefusalCopy(error, event));
      // A full or closed event changes what the page offers: read it again.
      if (error instanceof HTTPError && error.response.status === 409) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.events.byId(slug, eventId) });
      }
    },
  });
}
