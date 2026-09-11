import { z } from 'zod';
import { optionalEmailField, phoneField, firstNameField, lastNameField } from '@/shared/lib/validations';

export const publicBookingSchema = z.object({
  client_first_name: firstNameField,
  client_last_name: lastNameField,
  client_phone: phoneField,
  client_email: optionalEmailField,
  client_notes: z.string().max(2000).optional(),
});

export type PublicBookingFormData = z.infer<typeof publicBookingSchema>;
