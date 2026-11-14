import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  bookingSeriesDetailResponseSchema,
  createBookingSeriesResponseSchema,
  endBookingSeriesResponseSchema,
} from '../schemas/bookingSeries.schema';
import type {
  BookingSeriesDetailResponse,
  CreateBookingSeriesRequest,
  CreateBookingSeriesResponse,
  EndBookingSeriesResponse,
} from '@/shared/types/api.types';

export const bookingSeriesApi = {
  /** `idempotencyKey`: see `useIdempotentMutation` — a retry must not create the series twice. */
  create: (
    complexId: string,
    data: CreateBookingSeriesRequest,
    idempotencyKey: string,
  ): Promise<CreateBookingSeriesResponse> =>
    api
      .post(`complexes/${complexId}/booking-series`, { json: data, headers: { 'Idempotency-Key': idempotencyKey } })
      .json()
      .then(parseWith(createBookingSeriesResponseSchema, 'bookingSeriesApi.create')),

  get: (complexId: string, seriesId: string, signal?: AbortSignal): Promise<BookingSeriesDetailResponse> =>
    api
      .get(`complexes/${complexId}/booking-series/${seriesId}`, withSignal(signal))
      .json()
      .then(parseWith(bookingSeriesDetailResponseSchema, 'bookingSeriesApi.get')),

  end: (complexId: string, seriesId: string, fromDate: string): Promise<EndBookingSeriesResponse> =>
    api
      .post(`complexes/${complexId}/booking-series/${seriesId}/end`, { json: { from_date: fromDate } })
      .json()
      .then(parseWith(endBookingSeriesResponseSchema, 'bookingSeriesApi.end')),
};
