import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  instagramConnectResponseSchema,
  instagramDisconnectResponseSchema,
  instagramStatusSchema,
  type InstagramStatus,
} from '../schemas/instagram.schema';
import {
  instagramStoriesResponseSchema,
  instagramStorySettingsResponseSchema,
  type InstagramStoriesResponse,
  type InstagramStorySettings,
} from '../schemas/instagramStories.schema';

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

  /** Story settings and the last stories. 501 when the connection is not offered to this owner. */
  getStories: (complexId: string, signal?: AbortSignal): Promise<InstagramStoriesResponse> =>
    api
      .get(`complexes/${complexId}/instagram/stories`, withSignal(signal))
      .json()
      .then(parseWith(instagramStoriesResponseSchema, 'instagramApi.getStories')),

  /** Replaces the four settings; 422 names the field the server refused. */
  updateStories: (complexId: string, settings: InstagramStorySettings): Promise<{ settings: InstagramStorySettings }> =>
    api
      .put(`complexes/${complexId}/instagram/stories`, { json: settings })
      .json()
      .then(parseWith(instagramStorySettingsResponseSchema, 'instagramApi.updateStories')),
};
