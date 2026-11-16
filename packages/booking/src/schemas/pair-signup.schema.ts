import '../lib/zodConfig';
import { z } from 'zod';
import { BOOKING_MESSAGES } from '../messages';
import { optionalEmailField, phoneField, firstNameField, lastNameField } from '../lib/validations';

/**
 * The pair sign-up form: the payer's name and phone are required, email is
 * optional, and the partner's name and phone are required. One phone cannot
 * stand for both players.
 */
export const pairSignUpSchema = z
  .object({
    first_name: firstNameField,
    last_name: lastNameField,
    phone: phoneField,
    email: optionalEmailField,
    partner: z.object({
      first_name: firstNameField,
      last_name: lastNameField,
      phone: phoneField,
    }),
  })
  .refine((data) => data.phone !== data.partner.phone, {
    path: ['partner', 'phone'],
    message: BOOKING_MESSAGES.tournaments.partnerPhoneSame,
  });

export type PairSignUpFormData = z.infer<typeof pairSignUpSchema>;
