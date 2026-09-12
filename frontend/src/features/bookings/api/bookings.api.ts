import api, { withSignal } from '@/shared/lib/ky';
import type {
  BookingsListResponse,
  BookingDetailResponse,
  CreateBookingRequest,
  CancelBookingRequest,
  CancelBookingResponse,
  ConfirmPaymentRequest,
  ConfirmPaymentResponse,
  ManualRefundResponse,
  UpdateBookingRequest,
  Booking,
} from '@/shared/types/api.types';
import {
  bookingsListResponseSchema,
  bookingDetailResponseSchema,
  bookingEnvelopeSchema,
  cancelBookingResponseSchema,
  confirmPaymentResponseSchema,
  manualRefundResponseSchema,
} from '@/shared/schemas';
import { parseWith } from '@/shared/lib/apiParse';

export const bookingsApi = {
  list: (
    complexId: string,
    date: string,
    status?: string,
    search?: string,
    signal?: AbortSignal,
  ): Promise<BookingsListResponse> => {
    const searchParams: Record<string, string> = { date, limit: '200' };
    if (status) searchParams.status = status;
    if (search) searchParams.search = search;
    return api
      .get(`complexes/${complexId}/bookings`, { searchParams, ...withSignal(signal) })
      .json()
      .then(parseWith(bookingsListResponseSchema, 'bookingsApi.list'));
  },

  getById: (complexId: string, bookingId: string, signal?: AbortSignal): Promise<BookingDetailResponse> =>
    api
      .get(`complexes/${complexId}/bookings/${bookingId}`, withSignal(signal))
      .json()
      .then(parseWith(bookingDetailResponseSchema, 'bookingsApi.getById')),

  /** `idempotencyKey`: see `useIdempotentMutation` — a retry must not book the slot twice. */
  create: (complexId: string, data: CreateBookingRequest, idempotencyKey: string): Promise<{ booking: Booking }> =>
    api
      .post(`complexes/${complexId}/bookings`, { json: data, headers: { 'Idempotency-Key': idempotencyKey } })
      .json()
      .then(parseWith(bookingEnvelopeSchema, 'bookingsApi.create')),

  cancel: (complexId: string, bookingId: string, data?: CancelBookingRequest): Promise<CancelBookingResponse> =>
    api
      .post(`complexes/${complexId}/bookings/${bookingId}/cancel`, {
        json: data ?? {},
      })
      .json()
      .then(parseWith(cancelBookingResponseSchema, 'bookingsApi.cancel')),

  /** `idempotencyKey`: see `useIdempotentMutation` — a retry must not record the payment twice. */
  confirmPayment: (
    complexId: string,
    bookingId: string,
    data: ConfirmPaymentRequest,
    idempotencyKey: string,
  ): Promise<ConfirmPaymentResponse> =>
    api
      .post(`complexes/${complexId}/bookings/${bookingId}/confirm-payment`, {
        json: data,
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(confirmPaymentResponseSchema, 'bookingsApi.confirmPayment')),

  update: (complexId: string, bookingId: string, data: UpdateBookingRequest): Promise<{ booking: Booking }> =>
    api
      .put(`complexes/${complexId}/bookings/${bookingId}`, { json: data })
      .json()
      .then(parseWith(bookingEnvelopeSchema, 'bookingsApi.update')),

  /** `idempotencyKey`: see `useIdempotentMutation` — a retry must not refund twice. */
  markManualRefund: (complexId: string, bookingId: string, idempotencyKey: string): Promise<ManualRefundResponse> =>
    api
      .post(`complexes/${complexId}/bookings/${bookingId}/manual-refund`, {
        json: {},
        headers: { 'Idempotency-Key': idempotencyKey },
      })
      .json()
      .then(parseWith(manualRefundResponseSchema, 'bookingsApi.markManualRefund')),
};
