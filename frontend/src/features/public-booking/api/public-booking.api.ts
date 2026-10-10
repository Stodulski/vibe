import type { KyInstance } from 'ky';
import type { PublicBookingRequest, PublicCancelBookingRequest } from '@/shared/types/api.types';
import { parseWith } from '@/shared/lib/apiParse';
import {
  publicComplexResponseSchema,
  publicBookingResponseSchema,
  bookingStatusResponseSchema,
  cancelInfoResponseSchema,
  publicCancelBookingResponseSchema,
} from '@/shared/schemas/publicBooking.schema';
import { availabilityEnvelopeSchema } from '@/shared/schemas/availability.schema';
import { withSignal } from './client';

/**
 * The endpoints of the public booking flow, bound to the client that
 * `BookingRoot` created for its configured API base.
 */
export function createPublicBookingApi(client: KyInstance) {
  return {
    getComplex: (slug: string, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${slug}`, withSignal(signal))
        .json()
        .then(parseWith(publicComplexResponseSchema, 'publicBookingApi.getComplex')),

    getAvailability: (slug: string, date: string, duration: number, signal?: AbortSignal) =>
      client
        .get(`public/complexes/${slug}/availability`, {
          searchParams: { date, duration },
          ...withSignal(signal),
        })
        .json()
        .then(parseWith(availabilityEnvelopeSchema, 'publicBookingApi.getAvailability')),

    /**
     * `idempotencyKey` makes a retried submit replay the first answer instead
     * of creating a second booking — see `useIdempotentMutation`.
     */
    createBooking: (data: PublicBookingRequest, idempotencyKey: string) =>
      client
        .post('book', { json: data, headers: { 'Idempotency-Key': idempotencyKey } })
        .json()
        .then(parseWith(publicBookingResponseSchema, 'publicBookingApi.createBooking')),

    getBookingStatus: (token: string, signal?: AbortSignal) =>
      client
        .get('book/status', { searchParams: { token }, ...withSignal(signal) })
        .json()
        .then(parseWith(bookingStatusResponseSchema, 'publicBookingApi.getBookingStatus')),

    getCancelInfo: (token: string, signal?: AbortSignal) =>
      client
        .get('book/cancel-info', { searchParams: { token }, ...withSignal(signal) })
        .json()
        .then(parseWith(cancelInfoResponseSchema, 'publicBookingApi.getCancelInfo')),

    cancelBooking: (data: PublicCancelBookingRequest) =>
      client
        .post('book/cancel', { json: data })
        .json()
        .then(parseWith(publicCancelBookingResponseSchema, 'publicBookingApi.cancelBooking')),
  };
}

export type PublicBookingApi = ReturnType<typeof createPublicBookingApi>;
