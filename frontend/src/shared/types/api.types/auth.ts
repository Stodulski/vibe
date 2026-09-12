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
