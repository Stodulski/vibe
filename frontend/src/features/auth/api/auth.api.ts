import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import {
  authResponseSchema,
  currentUserResponseSchema,
  userEnvelopeSchema,
  googleSignInResponseSchema,
} from '@/shared/schemas/auth.schema';
import { messageResponseSchema } from '@/shared/schemas/envelope.schema';
import type {
  AuthResponse,
  CurrentUserResponse,
  LoginRequest,
  RegisterRequest,
  UpdateMeRequest,
  User,
  GoogleSignInResponse,
  GoogleCompleteRequest,
} from '@/shared/types/api.types';

export const authApi = {
  login: (data: LoginRequest): Promise<AuthResponse> =>
    api.post('auth/login', { json: data }).json().then(parseWith(authResponseSchema, 'authApi.login')),

  register: (data: RegisterRequest): Promise<{ message: string }> =>
    api.post('auth/register', { json: data }).json().then(parseWith(messageResponseSchema, 'authApi.register')),

  googleSignIn: (credential: string): Promise<GoogleSignInResponse> =>
    api
      .post('auth/google', { json: { credential } })
      .json()
      .then(parseWith(googleSignInResponseSchema, 'authApi.googleSignIn')),

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

  updateMe: (data: UpdateMeRequest): Promise<{ user: User }> =>
    api.put('auth/me', { json: data }).json().then(parseWith(userEnvelopeSchema, 'authApi.updateMe')),

  deleteAccount: (): Promise<{ message: string }> =>
    api.delete('auth/me').json().then(parseWith(messageResponseSchema, 'authApi.deleteAccount')),
};
