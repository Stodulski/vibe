import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  cancelCategorySchema,
  cancelTournamentSchema,
  matchEnvelopeSchema,
  tournamentDetailSchema,
  tournamentsListSchema,
  withdrawPairSchema,
} from '../schemas/tournaments.schema';
import type {
  CancelTournamentCategoryResponse,
  CancelTournamentResponse,
  ChargeCourtFeeRequest,
  CreateTournamentRequest,
  DrawGenerateRequest,
  DrawMoveRequest,
  MatchResultRequest,
  ScheduleMatchRequest,
  SetCourtFeeRequest,
  TournamentDetailResponse,
  TournamentMatchResponse,
  TournamentsListResponse,
  WithdrawPairResponse,
} from '@/shared/types/api.types';

const base = (complexId: string) => `complexes/${complexId}/tournaments`;
const category = (complexId: string, tournamentId: string, categoryId: string) =>
  `${base(complexId)}/${tournamentId}/categories/${categoryId}`;
const match = (complexId: string, tournamentId: string, matchId: string) =>
  `${base(complexId)}/${tournamentId}/matches/${matchId}`;

export const tournamentsApi = {
  list: (complexId: string, dateFrom: string, dateTo: string, signal?: AbortSignal): Promise<TournamentsListResponse> =>
    api
      .get(base(complexId), { searchParams: { date_from: dateFrom, date_to: dateTo }, ...withSignal(signal) })
      .json()
      .then(parseWith(tournamentsListSchema, 'tournamentsApi.list')),

  get: (complexId: string, tournamentId: string, signal?: AbortSignal): Promise<TournamentDetailResponse> =>
    api
      .get(`${base(complexId)}/${tournamentId}`, withSignal(signal))
      .json()
      .then(parseWith(tournamentDetailSchema, 'tournamentsApi.get')),

  create: (complexId: string, data: CreateTournamentRequest): Promise<TournamentDetailResponse> =>
    api.post(base(complexId), { json: data }).json().then(parseWith(tournamentDetailSchema, 'tournamentsApi.create')),

  cancel: (complexId: string, tournamentId: string): Promise<CancelTournamentResponse> =>
    api
      .post(`${base(complexId)}/${tournamentId}/cancel`)
      .json()
      .then(parseWith(cancelTournamentSchema, 'tournamentsApi.cancel')),

  cancelCategory: (
    complexId: string,
    tournamentId: string,
    categoryId: string,
  ): Promise<CancelTournamentCategoryResponse> =>
    api
      .post(`${category(complexId, tournamentId, categoryId)}/cancel`)
      .json()
      .then(parseWith(cancelCategorySchema, 'tournamentsApi.cancelCategory')),

  withdrawPair: (
    complexId: string,
    tournamentId: string,
    categoryId: string,
    registrationId: string,
  ): Promise<WithdrawPairResponse> =>
    api
      .post(`${category(complexId, tournamentId, categoryId)}/pairs/${registrationId}/withdraw`)
      .json()
      .then(parseWith(withdrawPairSchema, 'tournamentsApi.withdrawPair')),

  generateDraw: (
    complexId: string,
    tournamentId: string,
    categoryId: string,
    data: DrawGenerateRequest,
  ): Promise<TournamentDetailResponse> =>
    api
      .post(`${category(complexId, tournamentId, categoryId)}/draw`, { json: data })
      .json()
      .then(parseWith(tournamentDetailSchema, 'tournamentsApi.generateDraw')),

  moveDraw: (
    complexId: string,
    tournamentId: string,
    categoryId: string,
    data: DrawMoveRequest,
  ): Promise<TournamentDetailResponse> =>
    api
      .post(`${category(complexId, tournamentId, categoryId)}/draw/move`, { json: data })
      .json()
      .then(parseWith(tournamentDetailSchema, 'tournamentsApi.moveDraw')),

  schedule: (
    complexId: string,
    tournamentId: string,
    matchId: string,
    data: ScheduleMatchRequest,
  ): Promise<TournamentMatchResponse> =>
    api
      .put(`${match(complexId, tournamentId, matchId)}/schedule`, { json: data })
      .json()
      .then(parseWith(matchEnvelopeSchema, 'tournamentsApi.schedule')),

  unschedule: (complexId: string, tournamentId: string, matchId: string): Promise<TournamentMatchResponse> =>
    api
      .delete(`${match(complexId, tournamentId, matchId)}/schedule`)
      .json()
      .then(parseWith(matchEnvelopeSchema, 'tournamentsApi.unschedule')),

  setResult: (
    complexId: string,
    tournamentId: string,
    matchId: string,
    data: MatchResultRequest,
  ): Promise<TournamentDetailResponse> =>
    api
      .put(`${match(complexId, tournamentId, matchId)}/result`, { json: data })
      .json()
      .then(parseWith(tournamentDetailSchema, 'tournamentsApi.setResult')),

  setCourtFee: (
    complexId: string,
    tournamentId: string,
    categoryId: string,
    data: SetCourtFeeRequest,
  ): Promise<TournamentDetailResponse> =>
    api
      .put(`${category(complexId, tournamentId, categoryId)}/court-fee`, { json: data })
      .json()
      .then(parseWith(tournamentDetailSchema, 'tournamentsApi.setCourtFee')),

  chargeCourtFee: (
    complexId: string,
    tournamentId: string,
    matchId: string,
    side: 1 | 2,
    data: ChargeCourtFeeRequest,
  ): Promise<TournamentDetailResponse> =>
    api
      .put(`${match(complexId, tournamentId, matchId)}/court-fee/${String(side)}`, { json: data })
      .json()
      .then(parseWith(tournamentDetailSchema, 'tournamentsApi.chargeCourtFee')),

  undoCourtFee: (
    complexId: string,
    tournamentId: string,
    matchId: string,
    side: 1 | 2,
  ): Promise<TournamentDetailResponse> =>
    api
      .delete(`${match(complexId, tournamentId, matchId)}/court-fee/${String(side)}`)
      .json()
      .then(parseWith(tournamentDetailSchema, 'tournamentsApi.undoCourtFee')),
};
