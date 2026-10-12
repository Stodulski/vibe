import type { KyInstance } from 'ky';
import type { PublicBookingRequest, PublicCancelBookingRequest, WaitlistJoinRequest } from './types';
import { parseWith, type ErrorReporter } from '../lib/apiParse';
import {
  publicComplexResponseSchema,
  publicBookingResponseSchema,
  bookingStatusResponseSchema,
  cancelInfoResponseSchema,
  publicCancelBookingResponseSchema,
} from '../schemas/publicBooking.schema';
import { availabilityEnvelopeSchema, nextAvailableSchema } from '../schemas/availability.schema';
import { withSignal } from './client';

/**
 * The endpoints of the public booking flow, bound to the client that
 * `BookingRoot` created for its configured API base.
 */
export function createPublicBookingApi(client: KyInstance, report?: ErrorReporter) {
  return {
    getComplex: (slug: string, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${slug}`, withSignal(signal))
        .json()
        .then(parseWith(publicComplexResponseSchema, 'publicBookingApi.getComplex', report)),

    getAvailability: (slug: string, date: string, duration: number, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${slug}/availability`, {
          searchParams: { date, duration },
          ...withSignal(signal),
        })
        .json()
        .then(parseWith(availabilityEnvelopeSchema, 'publicBookingApi.getAvailability', report)),

    /** The first day after `from` with a free slot for `duration`, or `null` when the window has none. */
    getNextAvailable: (slug: string, from: string, duration: number | null, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${slug}/next-available`, {
          searchParams: duration === null ? { from } : { from, duration },
          ...withSignal(signal),
        })
        .json()
        .then(parseWith(nextAvailableSchema, 'publicBookingApi.getNextAvailable', report)),

    /** Puts the email on the waitlist for the day. The answer is an empty `204`. */
    joinWaitlist: (slug: string, data: WaitlistJoinRequest) =>
      client.post(`public/complexes/${slug}/waitlist`, { json: data }).then(() => undefined),

    /**
     * `idempotencyKey` makes a retried submit replay the first answer instead
     * of creating a second booking — see `useIdempotentMutation`.
     */
    createBooking: (data: PublicBookingRequest, idempotencyKey: string) =>
      client
        .post('book', { json: data, headers: { 'Idempotency-Key': idempotencyKey } })
        .json()
        .then(parseWith(publicBookingResponseSchema, 'publicBookingApi.createBooking', report)),

    getBookingStatus: (token: string, signal?: AbortSignal) =>
      client
        .get('book/status', { searchParams: { token }, ...withSignal(signal) })
        .json()
        .then(parseWith(bookingStatusResponseSchema, 'publicBookingApi.getBookingStatus', report)),

    getCancelInfo: (token: string, signal?: AbortSignal) =>
      client
        .get('book/cancel-info', { searchParams: { token }, ...withSignal(signal) })
        .json()
        .then(parseWith(cancelInfoResponseSchema, 'publicBookingApi.getCancelInfo', report)),

    cancelBooking: (data: PublicCancelBookingRequest) =>
      client
        .post('book/cancel', { json: data })
        .json()
        .then(parseWith(publicCancelBookingResponseSchema, 'publicBookingApi.cancelBooking', report)),
  };
}

export type PublicBookingApi = ReturnType<typeof createPublicBookingApi>;
