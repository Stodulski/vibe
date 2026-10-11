import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

// Free-text fields trim before any other check, so padding never counts as
// content: '   ' is empty for a required field, and a padded value is saved
// trimmed. Passwords are the exception (see `passwordField`).

export const emailField = z.string().trim().min(1, t.validation.emailRequired).pipe(z.email(t.validation.emailInvalid));

// A whitespace-only value trims to '' and matches the second branch, so it is
// treated as empty instead of failing as an invalid email.
export const optionalEmailField = emailField.optional().or(z.string().trim().length(0));

export const phoneField = z
  .string()
  .trim()
  .min(8, t.validation.phoneInvalid)
  .max(16, t.validation.maxChars15)
  .regex(/^\+[1-9]\d{7,14}$/, t.validation.phoneInvalid);

// Not trimmed on purpose: a leading or trailing space can be part of a password.
export const passwordField = z.string().min(8, t.validation.minChars8).max(72, t.validation.maxChars72);

export const firstNameField = z.string().trim().min(1, t.validation.nameRequired).max(100, t.validation.maxChars100);

export const lastNameField = z.string().trim().min(1, t.validation.lastNameRequired).max(100, t.validation.maxChars100);
