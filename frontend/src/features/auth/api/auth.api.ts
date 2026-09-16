import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  authResponseSchema,
  currentUserResponseSchema,
  updateMeResponseSchema,
  googleSignInResponseSchema,
} from '@/shared/schemas/auth.schema';
import { messageResponseSchema } from '@/shared/schemas/envelope.schema';
import type {
  AuthResponse,
  CurrentUserResponse,
  LoginRequest,
  RegisterRequest,
  UpdateMeRequest,
  UpdateMeResponse,
  GoogleSignInResponse,
  GoogleExchangeRequest,
  GoogleCompleteRequest,
} from '@/shared/types/api.types';

export const authApi = {
  login: (data: LoginRequest): Promise<AuthResponse> =>
    api.post('auth/login', { json: data }).json().then(parseWith(authResponseSchema, 'authApi.login')),

  register: (data: RegisterRequest): Promise<{ message: string }> =>
    api.post('auth/register', { json: data }).json().then(parseWith(messageResponseSchema, 'authApi.register')),

  /**
   * Trades the single-use code from `/auth/google/return?code=…`, together
   * with the `g_csrf_token` cookie Google set on this origin, for a session.
   * The credential itself never touches the browser in redirect mode: Google
   * POSTs it straight to the API, which answers this endpoint with exactly
   * what `POST /auth/google` used to answer the popup flow with — hence the
   * same schema.
   */
  googleExchange: (data: GoogleExchangeRequest): Promise<GoogleSignInResponse> =>
    api
      .post('auth/google/exchange', { json: data })
      .json()
      .then(parseWith(googleSignInResponseSchema, 'authApi.googleExchange')),

  googleComplete: (data: GoogleCompleteRequest): Promise<AuthResponse> =>
    api
      .post('auth/google/complete', { json: data })
      .json()
      .then(parseWith(authResponseSchema, 'authApi.googleComplete')),

  logout: () => api.post('auth/logout').json(),

  verifyEmail: (token: string, signal?: AbortSignal): Promise<{ message: string }> =>
    api
      .post('auth/verify-email', { json: { token }, ...withSignal(signal) })
      .json()
      .then(parseWith(messageResponseSchema, 'authApi.verifyEmail')),

  resendVerification: (email: string): Promise<{ message: string }> =>
    api
      .post('auth/resend-verification', { json: { email } })
      .json()
      .then(parseWith(messageResponseSchema, 'authApi.resendVerification')),

  forgotPassword: (email: string, turnstileToken?: string): Promise<{ message: string }> =>
    api
      .post('auth/forgot-password', { json: turnstileToken ? { email, turnstile_token: turnstileToken } : { email } })
      .json()
      .then(parseWith(messageResponseSchema, 'authApi.forgotPassword')),

  resetPassword: (token: string, password: string): Promise<{ message: string }> =>
    api
      .post('auth/reset-password', { json: { token, password } })
      .json()
      .then(parseWith(messageResponseSchema, 'authApi.resetPassword')),

  getMe: (signal?: AbortSignal): Promise<CurrentUserResponse> =>
    api.get('auth/me', withSignal(signal)).json().then(parseWith(currentUserResponseSchema, 'authApi.getMe')),

  updateMe: (data: UpdateMeRequest): Promise<UpdateMeResponse> =>
    api.put('auth/me', { json: data }).json().then(parseWith(updateMeResponseSchema, 'authApi.updateMe')),

  deleteAccount: (): Promise<{ message: string }> =>
    api.delete('auth/me').json().then(parseWith(messageResponseSchema, 'authApi.deleteAccount')),

  /**
   * Public, token-authenticated — same posture as `resetPassword`. The token
   * proves control of the account's CURRENT address: `PUT /auth/me` emails
   * the link there, never to the new one, so a session cookie cannot
   * substitute for it. Confirming does not itself prove control of the new
   * address — only the ordinary verification email the backend sends there
   * afterward does. On success the backend also revokes every session for
   * the account, so this tab must sign in again too — see
   * `useConfirmEmailChange`.
   */
  confirmEmailChange: (token: string, signal?: AbortSignal): Promise<{ message: string }> =>
    api
      .post('auth/confirm-email-change', { json: { token }, ...withSignal(signal) })
      .json()
      .then(parseWith(messageResponseSchema, 'authApi.confirmEmailChange')),
};
