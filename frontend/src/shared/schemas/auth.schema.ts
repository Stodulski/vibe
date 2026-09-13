import { z } from 'zod';
import type {
  User,
  UserRole,
  AuthResponse,
  RefreshResponse,
  CurrentUserResponse,
  GoogleProfilePreview,
  GoogleNeedsProfileResponse,
  GoogleSignInResponse,
} from '@/shared/types/api.types';

// ─── Auth ───
//
// `LoginRequest`, `RegisterRequest`, `UpdateMeRequest` and `BlockSlotRequest`
// are request bodies the client sends, not responses it parses — no schema
// for them here (the audit finding this fixes is about unvalidated
// responses).

export const userRoleSchema = z.enum(['owner', 'client', 'superadmin']) satisfies z.ZodType<UserRole>;

export const userSchema = z
  .object({
    id: z.string(),
    email: z.string(),
    first_name: z.string(),
    last_name: z.string(),
    role: userRoleSchema,
    phone: z.string(),
    is_active: z.boolean(),
    email_verified: z.boolean(),
    created_at: z.string(),
    updated_at: z.string(),
  })
  .loose() satisfies z.ZodType<User>;

export const authResponseSchema = z
  .object({
    user: userSchema,
    csrf_token: z.string(),
  })
  .loose() satisfies z.ZodType<AuthResponse>;

export const refreshResponseSchema = z
  .object({
    csrf_token: z.string(),
  })
  .loose() satisfies z.ZodType<RefreshResponse>;

/**
 * `{ user, csrf_token }` — `authApi.getMe`. A page load bootstraps the session
 * from this answer instead of rotating the refresh token, so a missing
 * `csrf_token` is a malformed session, not an optional extra.
 */
export const currentUserResponseSchema = z
  .object({
    user: userSchema,
    csrf_token: z.string(),
  })
  .loose() satisfies z.ZodType<CurrentUserResponse>;

/** `{ user: User }` — `authApi.updateMe`. */
export const userEnvelopeSchema = z
  .object({
    user: userSchema,
  })
  .loose() satisfies z.ZodType<{ user: User }>;

const googleProfilePreviewSchema = z
  .object({
    email: z.string(),
    first_name: z.string(),
    last_name: z.string(),
  })
  .loose() satisfies z.ZodType<GoogleProfilePreview>;

const googleNeedsProfileResponseSchema = z
  .object({
    needs_profile: z.literal(true),
    profile_token: z.string(),
    profile: googleProfilePreviewSchema,
  })
  .loose() satisfies z.ZodType<GoogleNeedsProfileResponse>;

/** `POST /auth/google` — either shape from {@link GoogleSignInResponse}. */
export const googleSignInResponseSchema = z.union([
  authResponseSchema,
  googleNeedsProfileResponseSchema,
]) satisfies z.ZodType<GoogleSignInResponse>;
