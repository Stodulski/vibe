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

  create: (complexId: string, data: CreateBookingRequest): Promise<{ booking: Booking }> =>
    api
      .post(`complexes/${complexId}/bookings`, { json: data })
      .json()
      .then(parseWith(bookingEnvelopeSchema, 'bookingsApi.create')),

  cancel: (complexId: string, bookingId: string, data?: CancelBookingRequest): Promise<CancelBookingResponse> =>
    api
      .post(`complexes/${complexId}/bookings/${bookingId}/cancel`, {
        json: data ?? {},
      })
      .json()
      .then(parseWith(cancelBookingResponseSchema, 'bookingsApi.cancel')),

  confirmPayment: (
    complexId: string,
    bookingId: string,
    data: ConfirmPaymentRequest,
  ): Promise<ConfirmPaymentResponse> =>
    api
      .post(`complexes/${complexId}/bookings/${bookingId}/confirm-payment`, {
        json: data,
      })
      .json()
      .then(parseWith(confirmPaymentResponseSchema, 'bookingsApi.confirmPayment')),

  update: (complexId: string, bookingId: string, data: UpdateBookingRequest): Promise<{ booking: Booking }> =>
    api
      .put(`complexes/${complexId}/bookings/${bookingId}`, { json: data })
      .json()
      .then(parseWith(bookingEnvelopeSchema, 'bookingsApi.update')),

  markManualRefund: (complexId: string, bookingId: string): Promise<ManualRefundResponse> =>
    api
      .post(`complexes/${complexId}/bookings/${bookingId}/manual-refund`, { json: {} })
      .json()
      .then(parseWith(manualRefundResponseSchema, 'bookingsApi.markManualRefund')),
};
