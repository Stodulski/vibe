import { useMutation, useQueryClient } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../api/context';
import type { PublicTournamentCategory } from '../api/types';
import { BOOKING_MESSAGES } from '../messages';
import { tournamentRefusalCopy } from '../lib/tournamentRefusals';
import { pairDeclaresLevel, toDeclared } from '../lib/declaredPlayers';
import { isModality } from '../lib/traitLabels';
import { storeTournamentRegistrationToken } from '../lib/tournamentRegistrationStorage';
import { navigateTo } from '../lib/navigation';
import { queryKeys } from '../lib/queryKeys';
import type { PairSignUpFormData } from '../schemas/pair-signup.schema';

/**
 * Holds a spot for the pair and hands the payer to the Mercado Pago checkout.
 * A category with a modality also gets each player's gender and level; one
 * without, the request is the same as it was before the traits. `onMessage`
 * receives the text to show when the sign-up fails.
 */
export function useTournamentSignUp(
  slug: string,
  tournamentId: string,
  category: PublicTournamentCategory & { sport: string },
  onMessage: (message: string) => void,
) {
  const api = usePublicBookingApi();
  const queryClient = useQueryClient();
  const declares = isModality(category.modality) || pairDeclaresLevel(category);
  return useMutation({
    mutationFn: ({ data, token }: { data: PairSignUpFormData; token?: string | undefined }) => {
      const payer = declares ? toDeclared(data.gender, data.level) : {};
      const partner = declares ? toDeclared(data.partner.gender, data.partner.level) : {};
      return api.signUpPairForTournament(slug, tournamentId, category.id, {
        first_name: data.first_name,
        last_name: data.last_name,
        phone: data.phone,
        ...(data.email ? { email: data.email } : {}),
        ...payer,
        partner: {
          first_name: data.partner.first_name,
          last_name: data.partner.last_name,
          phone: data.partner.phone,
          ...partner,
        },
        ...(token ? { turnstile_token: token } : {}),
      });
    },
    onSuccess: (result) => {
      if (!result.mp_init_point) {
        onMessage(BOOKING_MESSAGES.publicBooking.paymentLinkError);
        return;
      }
      storeTournamentRegistrationToken(tournamentId, result.token);
      navigateTo(result.mp_init_point);
    },
    onError: (error) => {
      onMessage(tournamentRefusalCopy(error, category));
      // A full or closed category changes what the page offers: read it again.
      if (error instanceof HTTPError && error.response.status === 409) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.tournaments.byId(slug, tournamentId) });
      }
    },
  });
}
