import '../lib/zodConfig';
import { z } from 'zod';
import { emailField, phoneField, firstNameField, lastNameField } from '../lib/validations';

export const publicBookingSchema = z.object({
  client_first_name: firstNameField,
  client_last_name: lastNameField,
  client_phone: phoneField,
  client_email: emailField,
  client_notes: z.string().trim().max(2000).optional(),
});

export type PublicBookingFormData = z.infer<typeof publicBookingSchema>;
