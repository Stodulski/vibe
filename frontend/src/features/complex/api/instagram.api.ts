import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  instagramConnectResponseSchema,
  instagramDisconnectResponseSchema,
  instagramStatusSchema,
  type InstagramStatus,
} from '../schemas/instagram.schema';

export const instagramApi = {
  getStatus: (complexId: string, signal?: AbortSignal): Promise<InstagramStatus> =>
    api
      .get(`complexes/${complexId}/instagram`, withSignal(signal))
      .json()
      .then(parseWith(instagramStatusSchema, 'instagramApi.getStatus')),

  /**
   * Registers a single-use attempt and answers the Instagram consent page.
   * 501 when the feature is off on the server.
   */
  connect: (complexId: string): Promise<{ authorize_url: string }> =>
    api
      .post(`complexes/${complexId}/instagram/connect`)
      .json()
      .then(parseWith(instagramConnectResponseSchema, 'instagramApi.connect')),

  disconnect: (complexId: string): Promise<{ connected: boolean }> =>
    api
      .delete(`complexes/${complexId}/instagram`)
      .json()
      .then(parseWith(instagramDisconnectResponseSchema, 'instagramApi.disconnect')),
};
