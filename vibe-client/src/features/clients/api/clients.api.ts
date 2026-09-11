import api, { withSignal } from '@/shared/lib/ky';
import type { ClientsListResponse, ClientDetailResponse, UpdateClientRequest, Client } from '@/shared/types/api.types';
import { clientsListResponseSchema, clientDetailResponseSchema, clientEnvelopeSchema } from '@/shared/schemas';
import { parseWith } from '@/shared/lib/apiParse';

export const clientsApi = {
  list: (
    complexId: string,
    params?: { search?: string | undefined; cursor?: string | undefined; limit?: number },
    signal?: AbortSignal,
  ): Promise<ClientsListResponse> => {
    const searchParams: Record<string, string> = {};
    if (params?.search) searchParams.search = params.search;
    if (params?.cursor) searchParams.cursor = params.cursor;
    if (params?.limit) searchParams.limit = String(params.limit);
    return api
      .get(`complexes/${complexId}/clients`, { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(clientsListResponseSchema, 'clientsApi.list'));
  },

  getById: (complexId: string, clientId: string, signal?: AbortSignal): Promise<ClientDetailResponse> =>
    api
      .get(`complexes/${complexId}/clients/${clientId}`, withSignal(signal))
      .json()
      .then(parseWith(clientDetailResponseSchema, 'clientsApi.getById')),

  update: (complexId: string, clientId: string, data: UpdateClientRequest): Promise<{ client: Client }> =>
    api
      .put(`complexes/${complexId}/clients/${clientId}`, { json: data })
      .json()
      .then(parseWith(clientEnvelopeSchema, 'clientsApi.update')),
};
