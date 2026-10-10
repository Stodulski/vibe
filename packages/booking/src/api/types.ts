/**
 * The public booking payloads, written out by hand from the OpenAPI document
 * (`backend/internal/openapi/openapi.yaml`, as generated into the frontend's
 * `api.generated.ts`). The package does not import that generated file.
 *
 * Every schema in `../schemas` is checked against these types at compile time
 * (`exact<T>` and `satisfies`), and the app's booking-types test
 * asserts that these types and the generated ones accept each other, so a
 * field the backend renames or retypes fails a build instead of a page.
 *
 * Why not `z.infer`: Zod types every `.optional()` field as `field?: X |
 * undefined`, which `exactOptionalPropertyTypes` refuses where the document
 * says `field?: X`. The same reason the app keeps `exact` in `apiParse.ts`.
 */

// ─── Shared vocabularies ───

export type Weekday = 'monday' | 'tuesday' | 'wednesday' | 'thursday' | 'friday' | 'saturday' | 'sunday';

export type BookingStatus = 'pending' | 'confirmed' | 'cancelled' | 'completed' | 'no_show';

export type CollectionStatus = 'unpaid' | 'deposit_paid' | 'fully_paid';

/** The booking's own refund state. `partial` is set only by the system. */
export type BookingRefundStatus = 'none' | 'pending' | 'partial' | 'full';

export type Sport = 'padel' | 'tennis' | 'soccer' | 'basketball' | 'volleyball' | 'hockey' | 'pickleball';

export type CourtType = 'indoor' | 'outdoor' | 'semi_covered';

/** The durations the slot picker offers and the price bands are keyed on. */
export type DurationMinutes = 60 | 90 | 120;

export type Amenity =
  | 'parking'
  | 'changing_rooms'
  | 'showers'
  | 'bar'
  | 'racket_rental'
  | 'pro_shop'
  | 'wifi'
  | 'lockers'
  | 'lessons'
  | 'tournaments'
  | 'accessible'
  | 'match_recording';

/**
 * A closed vocabulary reopened for reading. The server can add a member in a
 * deploy this build has not seen; a response field typed this way keeps the
 * known members for autocomplete and says out loud that an unknown one can
 * arrive. Every consumer already falls back to rendering the raw value.
 */
export type Open<T extends string> = T | (string & {});

// ─── Complex and courts ───

/** The storefront projection of a complex: no owner, no MercadoPago account. */
export interface PublicComplex {
  id: string;
  name: string;
  slug: string;
  address: string;
  city: string;
  province: string;
  country_code: string;
  currency: string;
  phone: string;
  email?: string;
  logo_url?: string;
  cover_url?: string;
  description?: string;
  deposit_percentage: number;
  cancellation_hours: number;
  latitude?: number;
  longitude?: number;
  amenities: Amenity[];
  payments_enabled: boolean;
  version?: number;
}

export interface Schedule {
  id: string;
  complex_id: string;
  day: Weekday;
  open_time: string;
  close_time: string;
  is_closed: boolean;
}

export interface Court {
  id: string;
  complex_id: string;
  name: string;
  sport: Sport;
  court_type: CourtType;
  is_active: boolean;
  description?: string | null;
  created_at: string;
  updated_at: string;
  version?: number;
}

/**
 * `from_min`/`to_min` are the band as minutes from its weekday's midnight;
 * `to_min` may exceed 1440 for a band running into the next day.
 */
export interface CourtPrice {
  id: string;
  court_id: string;
  price: number;
  day_type: Weekday;
  time_from: string;
  time_to: string;
  from_min: number;
  to_min: number;
  version?: number;
}

export interface CourtWithPrices extends Court {
  prices: CourtPrice[];
}

export interface PublicComplexResponse {
  complex: PublicComplex;
  courts: CourtWithPrices[];
  schedules: Schedule[];
}

// ─── Availability ───

export interface AvailabilitySlot {
  start_time: string;
  end_time: string;
  /** Minutes from the window's midnight, unwrapped past 1440 for overnight venues. */
  start_min: number;
  duration_minutes: number;
  price: number;
  available: boolean;
}

export interface CourtAvailability {
  court_id: string;
  court_name: string;
  sport: Open<Sport>;
  court_type: Open<CourtType>;
  description?: string;
  slots: AvailabilitySlot[];
}

export interface AvailabilityData {
  date: string;
  day: Open<Weekday>;
  is_open: boolean;
  courts: CourtAvailability[];
}

// ─── Creating a booking ───

export interface PublicBookingRequest {
  complex_id: string;
  court_id: string;
  date: string;
  start_time: string;
  duration_minutes: DurationMinutes;
  client_first_name: string;
  client_last_name: string;
  client_phone: string;
  client_email?: string;
  client_notes?: string;
}

/** The confirmation projection of a booking. No primary key is sent. */
export interface PublicBookingEnvelope {
  status: BookingStatus;
  collection_status: CollectionStatus;
  refund_status: BookingRefundStatus;
  date: string;
  start_time: string;
  starts_at: string;
  ends_at: string;
  court_name: string;
  complex_name: string;
  price: number;
  deposit_amount: number;
}

export interface PublicBookingResponse {
  booking: PublicBookingEnvelope;
  /** The opaque, expiring credential for the status and cancel routes. */
  token: string;
  mp_init_point?: string;
  mp_preference_id?: string;
  service_fee?: number;
  total_client_pays?: number;
}

// ─── Status ───

export interface BookingStatusCancellation {
  can_cancel: boolean;
  /** Null when cancelling with a refund has no fixed deadline. */
  refund_deadline?: string | null;
  can_refund_now: boolean;
  cancellation_hours: number;
}

/**
 * Only `status`, `collection_status` and `refund_status` are guaranteed. A
 * server that predates the other fields answers with just those three, so the
 * page falls back instead of assuming the rest.
 */
export interface BookingStatusDetails {
  status: BookingStatus;
  collection_status: CollectionStatus;
  refund_status: BookingRefundStatus;
  complex_name?: string;
  complex_address?: string;
  complex_phone?: string;
  court_name?: string;
  sport?: Sport;
  court_type?: CourtType;
  date?: string;
  start_time?: string;
  starts_at?: string;
  ends_at?: string;
  duration_minutes?: DurationMinutes;
  price?: number;
  deposit_amount?: number;
  service_fee?: number;
  remaining_amount?: number;
  cancellation?: BookingStatusCancellation;
}

export interface BookingStatusResponse {
  booking: BookingStatusDetails;
}

// ─── Cancelling ───

/**
 * How the deposit would come back: `mercadopago` (automatically, to the
 * payment source), `manual` (a person at the venue returns it), or `none`.
 */
export type RefundMethod = 'mercadopago' | 'manual' | 'none';

export interface CancelInfoBooking {
  status: BookingStatus;
  date: string;
  start_time: string;
  court_name: string;
  complex_name: string;
  sport?: Sport;
  court_type?: CourtType;
  starts_at?: string;
  ends_at?: string;
  duration_minutes?: DurationMinutes;
  complex_address?: string;
}

export interface CancelInfoResponse {
  booking: CancelInfoBooking;
  can_cancel: boolean;
  can_refund: boolean;
  refund_method: RefundMethod;
  cancellation_hours: number;
  /** Centavos. Includes the service fee. */
  refund_amount?: number;
  /** Centavos. */
  paid_amount?: number;
}

export interface PublicCancelBookingRequest {
  token: string;
}

export type RefundStatus = 'none' | 'not_eligible' | 'issued' | 'already_issued' | 'queued' | 'manual';

/** `message` is the copy to show. `status` only chooses how it is presented. */
export interface RefundEnvelope {
  status: RefundStatus;
  message: string;
  amount?: number;
  manual_amount?: number;
  manual_message?: string;
}

export interface PublicCancelBookingResponse {
  booking: {
    status: BookingStatus;
    collection_status: CollectionStatus;
    refund_status: BookingRefundStatus;
  };
  refunded: boolean;
  refund: RefundEnvelope;
}
