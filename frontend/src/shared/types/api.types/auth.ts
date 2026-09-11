import type { BookingRefundStatus, BookingStatus, CollectionStatus } from './booking';

// ─── Auth ───

export interface User {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: UserRole;
  phone: string;
  is_active: boolean;
  email_verified: boolean;
  created_at: string;
  updated_at: string;
}

export type UserRole = 'owner' | 'client' | 'superadmin';

export interface LoginRequest {
  email: string;
  password: string;
  /** Cloudflare Turnstile token, sent only when the widget is configured (`VITE_TURNSTILE_SITE_KEY`) and solved. */
  turnstile_token?: string;
}

export interface RegisterRequest {
  first_name: string;
  last_name: string;
  email: string;
  password: string;
  phone: string;
  /** Cloudflare Turnstile token, sent only when the widget is configured (`VITE_TURNSTILE_SITE_KEY`) and solved. */
  turnstile_token?: string;
}

export interface AuthResponse {
  user: User;
  csrf_token: string;
}

export interface GoogleProfilePreview {
  email: string;
  first_name: string;
  last_name: string;
}

/** `POST /auth/google` when the Google account has no matching user yet. */
export interface GoogleNeedsProfileResponse {
  needs_profile: true;
  profile_token: string;
  profile: GoogleProfilePreview;
}

/** `POST /auth/google` — either an existing account logs in, or a new one needs a phone number first. */
export type GoogleSignInResponse = AuthResponse | GoogleNeedsProfileResponse;

export interface GoogleCompleteRequest {
  profile_token: string;
  phone: string;
  first_name?: string;
  last_name?: string;
}

export interface RefreshResponse {
  csrf_token: string;
}

/**
 * `GET /auth/me`: the user plus the CSRF token bound to the access token the
 * request authenticated with. The same shape sign-in answers with, named for
 * what it is on the boot path: the session a page load starts from.
 */
export type CurrentUserResponse = AuthResponse;

export type UpdateMeRequest = Partial<Pick<User, 'first_name' | 'last_name' | 'email' | 'phone'>> & {
  current_password?: string;
  new_password?: string;
};

/**
 * No `version`: the server dropped the optimistic-concurrency counter
 * (backend removed the version counter). A concurrent write is refused inside the
 * database by the booking status-reversal trigger, which the API surfaces as a
 * 409, so there is nothing for the client to send or reconcile.
 */
export interface UpdateBookingRequest {
  status?: BookingStatus;
  /**
   * The two payment axes are sent separately (backend split payment_status). The
   * server guards each with its own transition matrix: collection only moves
   * upward, and the refund axis may only leave `partial` for `full`.
   */
  collection_status?: CollectionStatus;
  refund_status?: BookingRefundStatus;
  notes?: string;
}

export interface BlockSlotRequest {
  date: string;
  start_time: string;
  end_time: string;
  reason?: string;
}
