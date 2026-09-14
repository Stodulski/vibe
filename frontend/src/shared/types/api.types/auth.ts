import type { Body, Ok, Spec } from './spec';

// ─── Auth ───

export type User = Spec<'User'>;

export type UserRole = User['role'];

export type LoginRequest = Body<'authLogin'>;

export type RegisterRequest = Body<'authRegister'>;

export type AuthResponse = Ok<'authLogin'>;

/** `POST /auth/google` when the Google account has no matching user yet. */
export type GoogleNeedsProfileResponse = Extract<Ok<'authGoogle'>, { needs_profile: true }>;

export type GoogleProfilePreview = GoogleNeedsProfileResponse['profile'];

/** `POST /auth/google` — either an existing account logs in, or a new one needs a phone number first. */
export type GoogleSignInResponse = Ok<'authGoogle'>;

/**
 * `POST /auth/google/exchange` — the single-use code the backend's redirect
 * handler put in `/auth/google/return?code=…`, plus the `g_csrf_token` cookie
 * Google set on this origin during the redirect hop.
 *
 * Both halves are required. The code alone proves only that *some* Google
 * sign-in happened, so an attacker could mint one with their own account and
 * hand the victim a `/auth/google/return?code=…` link to be silently logged
 * in as the attacker. The cookie is the other half of the double submit: the
 * backend stored its hash next to the code and refuses a mismatch, and the
 * attacker cannot set a cookie on this origin in the victim's browser.
 *
 * Hand-written, not generated: the endpoint is not in the backend's OpenAPI
 * document yet, so `api.generated.ts` has no `authGoogleExchange` operation
 * to derive it from. Swap this for `Body<'authGoogleExchange'>` once the
 * backend spec lands and `pnpm gen:api` can see it. Its *response* needs no
 * new type at all: the endpoint answers byte-identically to
 * `POST /auth/google`, so it reuses {@link GoogleSignInResponse} (and
 * `googleSignInResponseSchema`).
 */
export interface GoogleExchangeRequest {
  code: string;
  g_csrf_token: string;
}

export type GoogleCompleteRequest = Body<'authGoogleComplete'>;

export type RefreshResponse = Ok<'authRefresh'>;

/**
 * `GET /auth/me`: the user plus the CSRF token bound to the access token the
 * request authenticated with. The same shape sign-in answers with, named for
 * what it is on the boot path: the session a page load starts from.
 */
export type CurrentUserResponse = Ok<'authGetCurrentUser'>;

export type UpdateMeRequest = Body<'authUpdateCurrentUser'>;

/**
 * No `version`: the server dropped the optimistic-concurrency counter
 * (backend removed the version counter). A concurrent write is refused inside the
 * database by the booking status-reversal trigger, which the API surfaces as a
 * 409, so there is nothing for the client to send or reconcile.
 */
export type UpdateBookingRequest = Body<'bookingsUpdate'>;

export type BlockSlotRequest = Body<'courtsBlockSlot'>;
