import { z } from 'zod';
import { emailField, firstNameField, lastNameField, phoneField } from '@/shared/lib/validations';

export const personalInfoSchema = z.object({
  first_name: firstNameField,
  last_name: lastNameField,
  email: emailField,
  phone: phoneField,
});

export type PersonalInfoFormData = z.infer<typeof personalInfoSchema>;
