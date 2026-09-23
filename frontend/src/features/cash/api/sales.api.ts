import api, { withSignal } from '@/shared/lib/ky';
import type {
  SalesListResponse,
  SaleCreateResponse,
  SaleEnvelopeResponse,
  CreateSaleRequest,
  VoidSaleRequest,
} from '@/shared/types/api.types';
import { salesListResponseSchema, saleCreateResponseSchema, saleEnvelopeSchema } from '@/shared/schemas';
import { parseWith } from '@/shared/lib/apiParse';

export const salesApi = {
  list: (
    complexId: string,
    params?: { sessionId?: string | undefined; cursor?: string | undefined; limit?: number },
    signal?: AbortSignal,
  ): Promise<SalesListResponse> => {
    const searchParams: Record<string, string> = {};
    if (params?.sessionId) searchParams.session_id = params.sessionId;
    if (params?.cursor) searchParams.cursor = params.cursor;
    if (params?.limit) searchParams.limit = String(params.limit);
    return api
      .get(`complexes/${complexId}/sales`, { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(salesListResponseSchema, 'salesApi.list'));
  },

  getById: (complexId: string, saleId: string, signal?: AbortSignal): Promise<SaleEnvelopeResponse> =>
    api
      .get(`complexes/${complexId}/sales/${saleId}`, withSignal(signal))
      .json()
      .then(parseWith(saleEnvelopeSchema, 'salesApi.getById')),

  /** `idempotencyKey`: see `useIdempotentMutation` — a retry must not sell the same cart twice. */
  create: (complexId: string, data: CreateSaleRequest, idempotencyKey: string): Promise<SaleCreateResponse> =>
    api
      .post(`complexes/${complexId}/sales`, { json: data, headers: { 'Idempotency-Key': idempotencyKey } })
      .json()
      .then(parseWith(saleCreateResponseSchema, 'salesApi.create')),

  /** `idempotencyKey`: a retry must not void the sale twice. */
  void: (
    complexId: string,
    saleId: string,
    data: VoidSaleRequest,
    idempotencyKey: string,
  ): Promise<SaleEnvelopeResponse> =>
    api
      .post(`complexes/${complexId}/sales/${saleId}/void`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(saleEnvelopeSchema, 'salesApi.void')),
};
