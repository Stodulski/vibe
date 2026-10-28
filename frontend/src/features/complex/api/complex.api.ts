import api, { withSignal } from '@/shared/lib/ky';
import type {
  Complex,
  Schedule,
  CreateComplexRequest,
  UpdateComplexRequest,
  UpdateSchedulesRequest,
  PublicComplexResponse,
  MPConnectRequest,
  MPConnectResponse,
} from '@/shared/types/api.types';
import { parseWith } from '@/shared/lib/apiParse';
import {
  complexesListEnvelopeSchema,
  complexEnvelopeSchema,
  deleteComplexResponseSchema,
  schedulesEnvelopeSchema,
  slugAvailableResponseSchema,
} from '@/shared/schemas/complex.schema';
import { publicComplexResponseSchema, mpConnectResponseSchema } from '@/shared/schemas/publicBooking.schema';

export const complexApi = {
  list: (signal?: AbortSignal): Promise<{ complexes: Complex[] }> =>
    api.get('complexes', withSignal(signal)).json().then(parseWith(complexesListEnvelopeSchema, 'complexApi.list')),

  getById: (id: string, signal?: AbortSignal): Promise<{ complex: Complex }> =>
    api.get(`complexes/${id}`, withSignal(signal)).json().then(parseWith(complexEnvelopeSchema, 'complexApi.getById')),

  create: (data: CreateComplexRequest): Promise<{ complex: Complex }> =>
    api.post('complexes', { json: data }).json().then(parseWith(complexEnvelopeSchema, 'complexApi.create')),

  update: (id: string, data: UpdateComplexRequest): Promise<{ complex: Complex }> =>
    api.put(`complexes/${id}`, { json: data }).json().then(parseWith(complexEnvelopeSchema, 'complexApi.update')),

  // Deleting a venue takes its courts offline in the same transaction, and the
  // server reports how many. Nothing renders the count yet; the type carries it
  // so the contract is not silently narrower than the response.
  delete: (id: string): Promise<{ message: string; courts_deactivated: number }> =>
    api.delete(`complexes/${id}`).json().then(parseWith(deleteComplexResponseSchema, 'complexApi.delete')),

  updateSchedules: (id: string, data: UpdateSchedulesRequest): Promise<{ schedules: Schedule[] }> =>
    api
      .put(`complexes/${id}/schedules`, { json: data })
      .json()
      .then(parseWith(schedulesEnvelopeSchema, 'complexApi.updateSchedules')),

  // Same endpoint as `publicBookingApi.getComplex` — hits the public
  // projection, not the owner-facing `Complex` shape. Only `.schedules` is
  // consumed today (see `useSchedules`), but the type must match the wire
  // response so a future field read fails to compile instead of compiling
  // clean against a field the server never sends.
  getPublicComplex: (slug: string, signal?: AbortSignal): Promise<PublicComplexResponse> =>
    api
      .get(`public/complexes/${slug}`, withSignal(signal))
      .json()
      .then(parseWith(publicComplexResponseSchema, 'complexApi.getPublicComplex')),

  /**
   * Hands the server one prepared MercadoPago OAuth attempt before its link is
   * offered: the `state` nonce the authorization URL carries and the PKCE
   * verifier that matches its challenge. The server keeps it, single use, for
   * an hour, so the callback can finish in whatever tab, browser or installed
   * app MercadoPago sends the owner back to. 204, no body.
   */
  registerMPOAuthAttempt: (complexId: string, body: { state: string; code_verifier: string }): Promise<void> =>
    api.post(`complexes/${complexId}/mp/oauth-attempts`, { json: body }).then(() => undefined),

  /**
   * Exchanges the MercadoPago OAuth `code` for a linked seller account.
   *
   * Called once from the `/settings/mp/callback` page, always with the `state`
   * the callback came back with. With `state` alone the server uses the
   * verifier it stored through {@link complexApi.registerMPOAuthAttempt};
   * `code_verifier` is sent only when this browser still holds the attempt
   * locally (and by bundles older than the server-held attempt). Neither field
   * is on `MPConnectRequest`, the generated wire contract, yet.
   */
  connectMP: (
    complexId: string,
    body: MPConnectRequest & { code_verifier?: string; state?: string },
  ): Promise<MPConnectResponse> =>
    api
      .post(`complexes/${complexId}/mp/connect`, { json: body })
      .json()
      .then(parseWith(mpConnectResponseSchema, 'complexApi.connectMP')),

  /**
   * Is this public URL free, and if not, what is.
   *
   * Not under `complexes/` — the router gives everything after that path to
   * the `:id` wildcard, so the endpoint lives at the root and the query says
   * what it is about.
   *
   * `suggestion` comes back only when the asked-for slug is taken, and is
   * itself verified free; it is absent when fifty variants are gone, which is
   * the server declining to invent one rather than the client inventing it.
   */
  slugAvailable: (
    slug: string,
    signal?: AbortSignal,
  ): Promise<{
    slug: string;
    valid: boolean;
    available: boolean;
    suggestion?: string;
  }> =>
    api
      .get('slug-available', { searchParams: { slug }, ...withSignal(signal) })
      .json()
      .then(parseWith(slugAvailableResponseSchema, 'complexApi.slugAvailable')),
};
