import type { Open, RefundEnvelope, Sport } from './types';

// ─── Open-play events (canchas libres) ───

export type EventStatus = 'active' | 'cancelled';

/** An event as a visitor sees it on the complex's page. `end_time` is earlier than `start_time` when it runs past midnight. */
export interface PublicOpenPlayEvent {
  id: string;
  name: string;
  sport: Open<Sport>;
  /** The local day the event starts on (`YYYY-MM-DD`). */
  date: string;
  start_time: string;
  end_time: string;
  duration_minutes: number;
  capacity: number;
  /** `0` closes the sign-up. */
  spots_left: number;
  /** Centavos ARS per person. */
  price: number;
  /** Centavos ARS the person pays on top. */
  service_fee: number;
  /** Centavos ARS: `price` plus `service_fee`. */
  total_price: number;
  status: Open<EventStatus>;
  court_names: string[];
  complex_name?: string;
  /** A paid spot is refunded when cancelled at least this many hours ahead. */
  cancellation_hours?: number;
}

export interface PublicEventListResponse {
  events: PublicOpenPlayEvent[];
}

export interface PublicEventResponse {
  event: PublicOpenPlayEvent;
}

/** Sign-up is per person. The phone is required, the email optional. */
export interface PublicEventSignUpRequest {
  first_name: string;
  last_name: string;
  phone: string;
  email?: string;
}

/** `POST .../registrations`: the spot is held; pay at `mp_init_point`. */
export interface PublicEventSignUp {
  registration: {
    status: 'held';
    /** The spot is released if unpaid by then. */
    held_until: string;
  };
  /** The person's credential for the status and cancel routes. */
  token: string;
  event: PublicOpenPlayEvent;
  mp_init_point?: string;
  mp_preference_id?: string;
  service_fee: number;
  total_price: number;
}

export type EventRegistrationStatus = 'held' | 'confirmed' | 'cancelled' | 'expired';

export interface PublicEventCancellation {
  can_cancel: boolean;
  /** Whether cancelling now earns the payment back. */
  within_window: boolean;
  deadline: string;
}

export interface PublicEventRegistration {
  status: Open<EventRegistrationStatus>;
  first_name: string;
  last_name: string;
  price: number;
  service_fee: number;
  held_until?: string;
  event: PublicOpenPlayEvent;
  complex_name?: string;
  complex_address?: string;
  cancellation: PublicEventCancellation;
}

export interface PublicEventCancelRequest {
  token: string;
}

export interface PublicEventCancelResponse {
  registration: PublicEventRegistration;
  refunded: boolean;
  refund: RefundEnvelope;
}
