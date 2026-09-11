import api, { withSignal } from '@/shared/lib/ky';
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

export const publicBookingApi = {
  getComplex: (slug: string, signal?: AbortSignal) =>
    api
      .get(`public/complexes/${slug}`, withSignal(signal))
      .json()
      .then(parseWith(publicComplexResponseSchema, 'publicBookingApi.getComplex')),

  getAvailability: (slug: string, date: string, duration: number, signal?: AbortSignal) =>
    api
      .get(`public/complexes/${slug}/availability`, {
        searchParams: { date, duration },
        ...withSignal(signal),
      })
      .json()
      .then(parseWith(availabilityEnvelopeSchema, 'publicBookingApi.getAvailability')),

  createBooking: (data: PublicBookingRequest) =>
    api
      .post('book', { json: data })
      .json()
      .then(parseWith(publicBookingResponseSchema, 'publicBookingApi.createBooking')),

  getBookingStatus: (token: string, signal?: AbortSignal) =>
    api
      .get('book/status', { searchParams: { token }, ...withSignal(signal) })
      .json()
      .then(parseWith(bookingStatusResponseSchema, 'publicBookingApi.getBookingStatus')),

  getCancelInfo: (token: string, signal?: AbortSignal) =>
    api
      .get('book/cancel-info', { searchParams: { token }, ...withSignal(signal) })
      .json()
      .then(parseWith(cancelInfoResponseSchema, 'publicBookingApi.getCancelInfo')),

  cancelBooking: (data: PublicCancelBookingRequest) =>
    api
      .post('book/cancel', { json: data })
      .json()
      .then(parseWith(publicCancelBookingResponseSchema, 'publicBookingApi.cancelBooking')),
};
