import { useMutation, useQueryClient } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../api/context';
import { BOOKING_MESSAGES } from '../messages';
import { tournamentRefusalCopy } from '../lib/tournamentRefusals';
import { storeTournamentRegistrationToken } from '../lib/tournamentRegistrationStorage';
import { navigateTo } from '../lib/navigation';
import { queryKeys } from '../lib/queryKeys';
import type { PairSignUpFormData } from '../schemas/pair-signup.schema';

/**
 * Holds a spot for the pair and hands the payer to the Mercado Pago checkout.
 * `onMessage` receives the text to show when the sign-up fails.
 */
export function useTournamentSignUp(
  slug: string,
  tournamentId: string,
  categoryId: string,
  onMessage: (message: string) => void,
) {
  const api = usePublicBookingApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: PairSignUpFormData) =>
      api.signUpPairForTournament(slug, tournamentId, categoryId, {
        first_name: data.first_name,
        last_name: data.last_name,
        phone: data.phone,
        ...(data.email ? { email: data.email } : {}),
        partner: {
          first_name: data.partner.first_name,
          last_name: data.partner.last_name,
          phone: data.partner.phone,
        },
      }),
    onSuccess: (result) => {
      if (!result.mp_init_point) {
        onMessage(BOOKING_MESSAGES.publicBooking.paymentLinkError);
        return;
      }
      storeTournamentRegistrationToken(tournamentId, result.token);
      navigateTo(result.mp_init_point);
    },
    onError: (error) => {
      onMessage(tournamentRefusalCopy(error));
      // A full or closed category changes what the page offers: read it again.
      if (error instanceof HTTPError && error.response.status === 409) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.tournaments.byId(slug, tournamentId) });
      }
    },
  });
}
