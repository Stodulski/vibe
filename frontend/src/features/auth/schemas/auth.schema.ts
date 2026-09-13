import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { emailField, phoneField, passwordField, firstNameField, lastNameField } from '@/shared/lib/validations';

const t = ES_AR;

export const loginSchema = z.object({
  email: emailField,
  password: z.string().min(1, t.common.required),
});

export const registerSchema = z
  .object({
    first_name: firstNameField,
    last_name: lastNameField,
    email: emailField,
    phone: phoneField,
    password: passwordField,
    confirm_password: z.string().min(1, t.common.required),
  })
  .refine((d) => d.password === d.confirm_password, {
    message: t.auth.passwordMismatch,
    path: ['confirm_password'],
  });

export const forgotPasswordSchema = z.object({
  email: emailField,
});

export const resetPasswordSchema = z.object({
  password: passwordField,
});

export const googleCompleteSchema = z.object({
  first_name: firstNameField,
  last_name: lastNameField,
  phone: phoneField,
});

// ─── `location.state` shapes ───
//
// `history.state` isn't guaranteed to match what the app itself put there —
// it survives back/forward navigation, browser extensions, and a manually
// edited URL bar, none of which type-check against it. safeParse instead of
// trusting an `as` cast (see 06-auth-shared-tooling.md M9).

/** `location.state` on `/login`, set by `ProtectedRoute`'s redirect. */
export const loginRedirectStateSchema = z.object({
  from: z.object({ pathname: z.string() }),
});

/** `location.state` on `/verify-email-sent`, set by `useRegister`'s redirect. */
export const verifyEmailSentStateSchema = z.object({
  email: z.string(),
});

/** `location.state` on `/register/google`, set by `useGoogleSignIn`'s `needs_profile` redirect. */
export const googleCompleteStateSchema = z.object({
  profile_token: z.string(),
  profile: z.object({
    email: z.string(),
    first_name: z.string(),
    last_name: z.string(),
  }),
});

export type LoginDto = z.infer<typeof loginSchema>;
export type RegisterDto = z.infer<typeof registerSchema>;
export type ForgotPasswordDto = z.infer<typeof forgotPasswordSchema>;
export type ResetPasswordDto = z.infer<typeof resetPasswordSchema>;
export type GoogleCompleteDto = z.infer<typeof googleCompleteSchema>;
