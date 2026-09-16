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
 * The *response* needs no type of its own: the endpoint answers
 * byte-identically to `POST /auth/google`, so it reuses
 * {@link GoogleSignInResponse} (and `googleSignInResponseSchema`).
 */
export type GoogleExchangeRequest = Body<'authGoogleExchange'>;

export type GoogleCompleteRequest = Body<'authGoogleComplete'>;

export type RefreshResponse = Ok<'authRefresh'>;

/**
 * `GET /auth/me`: the user plus the CSRF token bound to the access token the
 * request authenticated with. The same shape sign-in answers with, named for
 * what it is on the boot path: the session a page load starts from.
 *
 * `pending_email` is the address of a not-yet-confirmed email change (see
 * `UpdateMeResponse`), or `null` when there is none.
 */
export type CurrentUserResponse = Ok<'authGetCurrentUser'>;

export type UpdateMeRequest = Body<'authUpdateCurrentUser'>;

/**
 * `PUT /auth/me`: the account as it stands right after the request.
 * `pending_email` is the address named by a just-created (or still-live
 * earlier) email-change request, or `null` — see `PersonalInfoForm` and
 * `useUpdateProfile`. Changing `email` never applies it immediately: `user`
 * still carries the old address here, and a confirmation link goes to it.
 *
 * `email_change` is `"none"` when the body did not ask for a different
 * address, `"requested"` when the pending request was saved and its
 * confirmation email enqueued, or `"failed"` when that save failed after
 * every other field in the request had already committed — `useUpdateProfile`
 * reads this instead of comparing the submitted and returned addresses.
 */
export type UpdateMeResponse = Ok<'authUpdateCurrentUser'>;

/**
 * No `version`: the server dropped the optimistic-concurrency counter
 * (backend removed the version counter). A concurrent write is refused inside the
 * database by the booking status-reversal trigger, which the API surfaces as a
 * 409, so there is nothing for the client to send or reconcile.
 */
export type UpdateBookingRequest = Body<'bookingsUpdate'>;

export type BlockSlotRequest = Body<'courtsBlockSlot'>;
