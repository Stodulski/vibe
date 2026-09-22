import api, { withSignal } from '@/shared/lib/ky';
import type {
  CashSessionsListResponse,
  CashSessionCurrentResponse,
  CashSessionDetailResponse,
  OpenCashSessionRequest,
  CloseCashSessionRequest,
  CreateCashMovementRequest,
  VoidCashMovementRequest,
  CashSession,
  CashMovement,
} from '@/shared/types/api.types';
import {
  cashSessionsListResponseSchema,
  cashSessionCurrentResponseSchema,
  cashSessionDetailResponseSchema,
  cashSessionEnvelopeSchema,
  cashMovementEnvelopeSchema,
} from '@/shared/schemas';
import { parseWith } from '@/shared/lib/apiParse';

export const cashApi = {
  list: (
    complexId: string,
    params?: { cursor?: string | undefined; limit?: number },
    signal?: AbortSignal,
  ): Promise<CashSessionsListResponse> => {
    const searchParams: Record<string, string> = {};
    if (params?.cursor) searchParams.cursor = params.cursor;
    if (params?.limit) searchParams.limit = String(params.limit);
    return api
      .get(`complexes/${complexId}/cash-sessions`, { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(cashSessionsListResponseSchema, 'cashApi.list'));
  },

  /** Rejects with a ky `HTTPError` (status 404) when no session is open — callers treat that as "closed". */
  current: (complexId: string, signal?: AbortSignal): Promise<CashSessionCurrentResponse> =>
    api
      .get(`complexes/${complexId}/cash-session`, withSignal(signal))
      .json()
      .then(parseWith(cashSessionCurrentResponseSchema, 'cashApi.current')),

  getById: (complexId: string, sessionId: string, signal?: AbortSignal): Promise<CashSessionDetailResponse> =>
    api
      .get(`complexes/${complexId}/cash-sessions/${sessionId}`, withSignal(signal))
      .json()
      .then(parseWith(cashSessionDetailResponseSchema, 'cashApi.getById')),

  /** `idempotencyKey`: see `useIdempotentMutation` — a retry must not open a second session. */
  open: (
    complexId: string,
    data: OpenCashSessionRequest,
    idempotencyKey: string,
  ): Promise<{ cash_session: CashSession }> =>
    api
      .post(`complexes/${complexId}/cash-sessions`, { json: data, headers: { 'Idempotency-Key': idempotencyKey } })
      .json()
      .then(parseWith(cashSessionEnvelopeSchema, 'cashApi.open')),

  /** `idempotencyKey`: a retry must not close (or attempt to close) the session twice. */
  close: (
    complexId: string,
    sessionId: string,
    data: CloseCashSessionRequest,
    idempotencyKey: string,
  ): Promise<{ cash_session: CashSession }> =>
    api
      .post(`complexes/${complexId}/cash-sessions/${sessionId}/close`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(cashSessionEnvelopeSchema, 'cashApi.close')),

  /** `idempotencyKey`: a retry must not record the movement twice. */
  createMovement: (
    complexId: string,
    sessionId: string,
    data: CreateCashMovementRequest,
    idempotencyKey: string,
  ): Promise<{ cash_movement: CashMovement }> =>
    api
      .post(`complexes/${complexId}/cash-sessions/${sessionId}/movements`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(cashMovementEnvelopeSchema, 'cashApi.createMovement')),

  /** `idempotencyKey`: a retry must not void the movement twice. */
  voidMovement: (
    complexId: string,
    sessionId: string,
    movementId: string,
    data: VoidCashMovementRequest,
    idempotencyKey: string,
  ): Promise<{ cash_movement: CashMovement }> =>
    api
      .post(`complexes/${complexId}/cash-sessions/${sessionId}/movements/${movementId}/void`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(cashMovementEnvelopeSchema, 'cashApi.voidMovement')),
};
