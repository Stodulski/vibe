import '../lib/zodConfig';
import { z } from 'zod';
import { optionalEmailField, phoneField, firstNameField, lastNameField } from '../lib/validations';

/** The sign-up form: name and phone required, email optional (without it no confirmation email is sent). */
export const eventSignUpSchema = z.object({
  first_name: firstNameField,
  last_name: lastNameField,
  phone: phoneField,
  email: optionalEmailField,
});

export type EventSignUpFormData = z.infer<typeof eventSignUpSchema>;
