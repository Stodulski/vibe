import { z } from 'zod';
import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES.validation;

// The field rules the booking form shares. Copied from the app's
// `shared/lib/validations.ts`, with the same Spanish messages, including the
// trim-first rule: padding never counts as content.

export const emailField = z.string().trim().min(1, t.emailRequired).pipe(z.email(t.emailInvalid));

// A whitespace-only value trims to '' and matches the second branch, so it is
// treated as empty instead of failing as an invalid email.
export const optionalEmailField = emailField.optional().or(z.string().trim().length(0));

export const phoneField = z
  .string()
  .trim()
  .min(8, t.phoneInvalid)
  .max(16, t.maxChars15)
  .regex(/^\+[1-9]\d{7,14}$/, t.phoneInvalid);

export const firstNameField = z.string().trim().min(1, t.nameRequired).max(100, t.maxChars100);

export const lastNameField = z.string().trim().min(1, t.lastNameRequired).max(100, t.maxChars100);
