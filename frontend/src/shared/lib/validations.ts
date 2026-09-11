import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const emailField = z.string().min(1, t.validation.emailRequired).pipe(z.email(t.validation.emailInvalid));

export const optionalEmailField = emailField.optional().or(z.literal(''));

export const phoneField = z
  .string()
  .min(8, t.validation.phoneInvalid)
  .max(16, t.validation.maxChars15)
  .regex(/^\+[1-9]\d{7,14}$/, t.validation.phoneInvalid);

export const passwordField = z.string().min(8, t.validation.minChars8).max(72, t.validation.maxChars72);

export const firstNameField = z.string().min(1, t.validation.nameRequired).max(100, t.validation.maxChars100);

export const lastNameField = z.string().min(1, t.validation.lastNameRequired).max(100, t.validation.maxChars100);
