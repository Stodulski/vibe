import type { KyInstance } from 'ky';
import type { PublicPairCancelRequest, PublicPairSignUpRequest } from './types';
import { parseWith, type ErrorReporter } from '../lib/apiParse';
import {
  publicPairCancelResponseSchema,
  publicPairRegistrationSchema,
  publicPairSignUpSchema,
  publicTournamentDetailSchema,
  publicTournamentListResponseSchema,
} from '../schemas/publicTournaments.schema';
import { withSignal } from './client';

const enc = encodeURIComponent;

/** The tournament endpoints of the public flow, merged into `createPublicBookingApi`. */
export function createPublicTournamentsApi(client: KyInstance, report?: ErrorReporter) {
  return {
    /** The tournaments a visitor can follow or sign up for, soonest first. */
    listTournaments: (slug: string, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${enc(slug)}/tournaments`, withSignal(signal))
        .json()
        .then(parseWith(publicTournamentListResponseSchema, 'publicBookingApi.listTournaments', report)),

    /** One tournament with its groups, standings, bracket and schedule. A cancelled one is answered too. */
    getTournament: (slug: string, tournamentId: string, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${enc(slug)}/tournaments/${enc(tournamentId)}`, withSignal(signal))
        .json()
        .then(parseWith(publicTournamentDetailSchema, 'publicBookingApi.getTournament', report)),

    /**
     * Holds a spot for one pair and opens the checkout (`mp_init_point`). Not
     * idempotent: a retry takes a second spot, which the server refuses for
     * the same phones.
     */
    signUpPairForTournament: (slug: string, tournamentId: string, categoryId: string, data: PublicPairSignUpRequest) =>
      client
        .post(`public/complexes/${enc(slug)}/tournaments/${enc(tournamentId)}/categories/${enc(categoryId)}/pairs`, {
          json: data,
        })
        .json()
        .then(parseWith(publicPairSignUpSchema, 'publicBookingApi.signUpPairForTournament', report)),

    getTournamentRegistration: (token: string, signal?: AbortSignal) =>
      client
        .get('public/tournament-registrations/status', { searchParams: { token }, ...withSignal(signal) })
        .json()
        .then(parseWith(publicPairRegistrationSchema, 'publicBookingApi.getTournamentRegistration', report)),

    cancelTournamentRegistration: (data: PublicPairCancelRequest) =>
      client
        .post('public/tournament-registrations/cancel', { json: data })
        .json()
        .then(parseWith(publicPairCancelResponseSchema, 'publicBookingApi.cancelTournamentRegistration', report)),
  };
}
