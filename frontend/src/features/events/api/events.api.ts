import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  cancelEventEnvelopeSchema,
  eventDetailEnvelopeSchema,
  eventEnvelopeSchema,
  eventsListEnvelopeSchema,
} from '../schemas/events.schema';
import type {
  CancelOpenPlayEventResponse,
  CreateOpenPlayEventRequest,
  OpenPlayEvent,
  OpenPlayEventDetailResponse,
} from '@/shared/types/api.types';

// The owner routes are `/open-play-events`: `/complexes/{id}/events` is the
// realtime stream.
export const eventsApi = {
  list: (
    complexId: string,
    dateFrom: string,
    dateTo: string,
    signal?: AbortSignal,
  ): Promise<{ events: OpenPlayEvent[] }> =>
    api
      .get(`complexes/${complexId}/open-play-events`, {
        searchParams: { date_from: dateFrom, date_to: dateTo },
        ...withSignal(signal),
      })
      .json()
      .then(parseWith(eventsListEnvelopeSchema, 'eventsApi.list')),

  get: (complexId: string, eventId: string, signal?: AbortSignal): Promise<OpenPlayEventDetailResponse> =>
    api
      .get(`complexes/${complexId}/open-play-events/${eventId}`, withSignal(signal))
      .json()
      .then(parseWith(eventDetailEnvelopeSchema, 'eventsApi.get')),

  create: (complexId: string, data: CreateOpenPlayEventRequest): Promise<{ event: OpenPlayEvent }> =>
    api
      .post(`complexes/${complexId}/open-play-events`, { json: data })
      .json()
      .then(parseWith(eventEnvelopeSchema, 'eventsApi.create')),

  cancel: (complexId: string, eventId: string): Promise<CancelOpenPlayEventResponse> =>
    api
      .post(`complexes/${complexId}/open-play-events/${eventId}/cancel`)
      .json()
      .then(parseWith(cancelEventEnvelopeSchema, 'eventsApi.cancel')),
};
