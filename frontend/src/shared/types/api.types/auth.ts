import type { Body, Ok, Spec } from './spec';

// ─── Auth ───

export type User = Spec<'User'>;

export type UserRole = User['role'];

export type LoginRequest = Body<'authLogin'>;

export type RegisterRequest = Body<'authRegister'>;

export type AuthResponse = Ok<'authLogin'>;

/** `POST /auth/google/finish` when the Google account has no matching user yet. */
export type GoogleNeedsProfileResponse = Extract<Ok<'authGoogleFinish'>, { needs_profile: true }>;

export type GoogleProfilePreview = GoogleNeedsProfileResponse['profile'];

/** `POST /auth/google/finish` — either an existing account logs in, or a new one needs a phone number first. */
export type GoogleSignInResponse = Ok<'authGoogleFinish'>;

/**
 * `POST /auth/google/finish` — the authorization code and state Google
 * appended to `/auth/google/callback?code=…&state=…` after the visitor
 * chose an account.
 */
export type GoogleFinishRequest = Body<'authGoogleFinish'>;

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
