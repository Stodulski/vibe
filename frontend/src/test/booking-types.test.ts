import { describe, expect, it } from 'vitest';
import type * as Booking from '@vibe/booking/types';
import type * as App from '@/shared/types/api.types';

// The booking package writes its payload types by hand. Each pair below must
// accept the other, so a field the backend renames, retypes or drops fails
// `tsc` here instead of at runtime in a booking page.

type Mutual<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
type Assert<T extends true> = T;

export type BookingTypeChecks = [
  Assert<Mutual<Booking.Weekday, App.DayOfWeek>>,
  Assert<Mutual<Booking.BookingStatus, App.BookingStatus>>,
  Assert<Mutual<Booking.CollectionStatus, App.CollectionStatus>>,
  Assert<Mutual<Booking.BookingRefundStatus, App.BookingRefundStatus>>,
  Assert<Mutual<Booking.Sport, App.Sport>>,
  Assert<Mutual<Booking.CourtType, App.CourtType>>,
  Assert<Mutual<Booking.DurationMinutes, App.DurationMinutes>>,
  Assert<Mutual<Booking.Amenity, App.Amenity>>,
  Assert<Mutual<Booking.PublicComplex, App.PublicComplex>>,
  Assert<Mutual<Booking.Schedule, App.Schedule>>,
  Assert<Mutual<Booking.Court, App.Court>>,
  Assert<Mutual<Booking.CourtPrice, App.CourtPrice>>,
  Assert<Mutual<Booking.CourtWithPrices, App.CourtWithPrices>>,
  Assert<Mutual<Booking.PublicComplexResponse, App.PublicComplexResponse>>,
  Assert<Mutual<Booking.AvailabilitySlot, App.AvailabilitySlot>>,
  Assert<Mutual<Booking.CourtAvailability, App.CourtAvailability>>,
  Assert<Mutual<Booking.AvailabilityData, App.AvailabilityData>>,
  // Request bodies are the app's `Body<>`, widened by `Loose` for zod inputs. The
  // package sends exact values, so the one direction that matters is that the
  // document accepts everything the package can send.
  Assert<[Booking.PublicBookingRequest] extends [App.PublicBookingRequest] ? true : false>,
  Assert<Mutual<Booking.PublicBookingEnvelope, App.PublicBookingEnvelope>>,
  Assert<Mutual<Booking.PublicBookingResponse, App.PublicBookingResponse>>,
  Assert<Mutual<Booking.BookingStatusCancellation, App.BookingStatusCancellation>>,
  Assert<Mutual<Booking.BookingStatusDetails, App.BookingStatusDetails>>,
  Assert<Mutual<Booking.BookingStatusResponse, App.BookingStatusResponse>>,
  Assert<Mutual<Booking.RefundMethod, App.RefundMethod>>,
  Assert<Mutual<Booking.CancelInfoResponse, App.CancelInfoResponse>>,
  Assert<Mutual<Booking.PublicCancelBookingRequest, App.PublicCancelBookingRequest>>,
  Assert<Mutual<Booking.RefundStatus, App.RefundStatus>>,
  Assert<Mutual<Booking.RefundEnvelope, App.RefundEnvelope>>,
  Assert<Mutual<Booking.PublicCancelBookingResponse, App.PublicCancelBookingResponse>>,
];

describe('booking package types', () => {
  it('stay mutually assignable with the generated API types', () => {
    // The assertions above are checked by the type checker; the runtime check
    // only confirms the module is part of the suite.
    const checks: BookingTypeChecks | undefined = undefined;
    expect(checks).toBeUndefined();
  });
});
