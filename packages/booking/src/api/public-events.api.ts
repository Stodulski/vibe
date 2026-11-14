import type { KyInstance } from 'ky';
import type { PublicEventCancelRequest, PublicEventSignUpRequest } from './types';
import { parseWith, type ErrorReporter } from '../lib/apiParse';
import {
  publicEventCancelResponseSchema,
  publicEventListResponseSchema,
  publicEventRegistrationSchema,
  publicEventResponseSchema,
  publicEventSignUpSchema,
} from '../schemas/publicEvents.schema';
import { withSignal } from './client';

/** The open-play event endpoints of the public flow, merged into `createPublicBookingApi`. */
export function createPublicEventsApi(client: KyInstance, report?: ErrorReporter) {
  return {
    /** The open-play events a visitor can still sign up for, soonest first. */
    listEvents: (slug: string, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${encodeURIComponent(slug)}/events`, withSignal(signal))
        .json()
        .then(parseWith(publicEventListResponseSchema, 'publicBookingApi.listEvents', report)),

    /** One event for its shareable page. A cancelled event is still answered, as `cancelled`. */
    getEvent: (slug: string, eventId: string, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${encodeURIComponent(slug)}/events/${encodeURIComponent(eventId)}`, withSignal(signal))
        .json()
        .then(parseWith(publicEventResponseSchema, 'publicBookingApi.getEvent', report)),

    /**
     * Holds a spot for one person and opens the checkout (`mp_init_point`).
     * Not idempotent: a retry takes a second spot, which the server refuses
     * for the same phone.
     */
    signUpForEvent: (slug: string, eventId: string, data: PublicEventSignUpRequest) =>
      client
        .post(`public/complexes/${encodeURIComponent(slug)}/events/${encodeURIComponent(eventId)}/registrations`, { json: data })
        .json()
        .then(parseWith(publicEventSignUpSchema, 'publicBookingApi.signUpForEvent', report)),

    getEventRegistration: (token: string, signal?: AbortSignal) =>
      client
        .get('public/event-registrations/status', { searchParams: { token }, ...withSignal(signal) })
        .json()
        .then(parseWith(publicEventRegistrationSchema, 'publicBookingApi.getEventRegistration', report)),

    cancelEventRegistration: (data: PublicEventCancelRequest) =>
      client
        .post('public/event-registrations/cancel', { json: data })
        .json()
        .then(parseWith(publicEventCancelResponseSchema, 'publicBookingApi.cancelEventRegistration', report)),
  };
}
