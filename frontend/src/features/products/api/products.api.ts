import api, { withSignal } from '@/shared/lib/ky';
import type {
  ProductsListResponse,
  ProductEnvelopeResponse,
  ProductStockMovementsResponse,
  ProductStockWriteResponse,
  CreateProductRequest,
  UpdateProductRequest,
  RestockProductRequest,
  AdjustProductRequest,
} from '@/shared/types/api.types';
import {
  productsListResponseSchema,
  productEnvelopeSchema,
  productStockMovementsResponseSchema,
  productStockWriteResponseSchema,
} from '@/shared/schemas';
import { parseWith } from '@/shared/lib/apiParse';

export const productsApi = {
  /** Unpaginated — a shop's catalog is bounded (`productsList`'s own doc comment). */
  list: (
    complexId: string,
    params?: { active?: boolean | undefined },
    signal?: AbortSignal,
  ): Promise<ProductsListResponse> => {
    const searchParams: Record<string, string> = {};
    if (params?.active !== undefined) searchParams.active = String(params.active);
    return api
      .get(`complexes/${complexId}/products`, { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(productsListResponseSchema, 'productsApi.list'));
  },

  getById: (complexId: string, productId: string, signal?: AbortSignal): Promise<ProductEnvelopeResponse> =>
    api
      .get(`complexes/${complexId}/products/${productId}`, withSignal(signal))
      .json()
      .then(parseWith(productEnvelopeSchema, 'productsApi.getById')),

  /** `idempotencyKey`: see `useIdempotentMutation` — a retry must not add the product twice. */
  create: (complexId: string, data: CreateProductRequest, idempotencyKey: string): Promise<ProductEnvelopeResponse> =>
    api
      .post(`complexes/${complexId}/products`, { json: data, headers: { 'Idempotency-Key': idempotencyKey } })
      .json()
      .then(parseWith(productEnvelopeSchema, 'productsApi.create')),

  /**
   * `idempotencyKey`: a retry must not apply the same patch twice.
   * `data.version`, when present, guards optimistic concurrency the same way
   * `useUpdateCourt` sends a court's `version` in the body — the API also
   * accepts `If-Match`, unused here since the body field alone is enough.
   */
  update: (
    complexId: string,
    productId: string,
    data: UpdateProductRequest,
    idempotencyKey: string,
  ): Promise<ProductEnvelopeResponse> =>
    api
      .patch(`complexes/${complexId}/products/${productId}`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(productEnvelopeSchema, 'productsApi.update')),

  /** `idempotencyKey`: a retry must not deliver (and pay for) the stock twice. */
  restock: (
    complexId: string,
    productId: string,
    data: RestockProductRequest,
    idempotencyKey: string,
  ): Promise<ProductStockWriteResponse> =>
    api
      .post(`complexes/${complexId}/products/${productId}/restock`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(productStockWriteResponseSchema, 'productsApi.restock')),

  /** `idempotencyKey`: a retry must not apply the same correction twice. */
  adjust: (
    complexId: string,
    productId: string,
    data: AdjustProductRequest,
    idempotencyKey: string,
  ): Promise<ProductStockWriteResponse> =>
    api
      .post(`complexes/${complexId}/products/${productId}/adjustments`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(productStockWriteResponseSchema, 'productsApi.adjust')),

  listStockMovements: (
    complexId: string,
    productId: string,
    params?: { cursor?: string | undefined; limit?: number },
    signal?: AbortSignal,
  ): Promise<ProductStockMovementsResponse> => {
    const searchParams: Record<string, string> = {};
    if (params?.cursor) searchParams.cursor = params.cursor;
    if (params?.limit) searchParams.limit = String(params.limit);
    return api
      .get(`complexes/${complexId}/products/${productId}/stock-movements`, { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(productStockMovementsResponseSchema, 'productsApi.listStockMovements'));
  },
};
