import { z } from 'zod';
import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES.validation;

// The field rules the booking form shares. Copied from the app's
// `shared/lib/validations.ts`, with the same Spanish messages.

export const emailField = z.string().min(1, t.emailRequired).pipe(z.email(t.emailInvalid));

export const optionalEmailField = emailField.optional().or(z.literal(''));

export const phoneField = z
  .string()
  .min(8, t.phoneInvalid)
  .max(16, t.maxChars15)
  .regex(/^\+[1-9]\d{7,14}$/, t.phoneInvalid);

export const firstNameField = z.string().min(1, t.nameRequired).max(100, t.maxChars100);

export const lastNameField = z.string().min(1, t.lastNameRequired).max(100, t.maxChars100);
