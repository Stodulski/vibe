import '../lib/zodConfig';
import { z } from 'zod';
import type { PublicTournamentCategory } from '../api/types';
import { BOOKING_MESSAGES } from '../messages';
import { optionalEmailField, phoneField, firstNameField, lastNameField } from '../lib/validations';
import { pairIssues, toDeclared } from '../lib/declaredPlayers';
import { isModality } from '../lib/traitLabels';

const d = BOOKING_MESSAGES.declaration;

/**
 * The fields of the pair sign-up form. The payer's name and phone are required,
 * email is optional, and the partner's name and phone are required. Each
 * player's gender and level are strings until the person chooses them, and they
 * are asked only for a category that has a modality.
 */
const pairFields = z.object({
  first_name: firstNameField,
  last_name: lastNameField,
  phone: phoneField,
  email: optionalEmailField,
  gender: z.string(),
  level: z.string(),
  partner: z.object({
    first_name: firstNameField,
    last_name: lastNameField,
    phone: phoneField,
    gender: z.string(),
    level: z.string(),
  }),
});

export type PairSignUpFormData = z.infer<typeof pairFields>;

type PairCategory = Pick<PublicTournamentCategory, 'modality' | 'level' | 'suma'>;

function requireDeclaration(ctx: z.RefinementCtx, path: string[], gender: string, level: string): void {
  if (gender !== 'men' && gender !== 'women') {
    ctx.addIssue({ code: 'custom', path: [...path, 'gender'], message: d.genderRequired });
  }
  if (!/^[1-9]$/.test(level)) {
    ctx.addIssue({ code: 'custom', path: [...path, 'level'], message: d.levelRequired });
  }
}

/**
 * The pair sign-up form for one category. A category with a modality needs each
 * player's gender and level, and both must fit its rules (the same rules the
 * server checks). A category without one takes the pair as it is.
 */
export function pairSignUpSchema(category: PairCategory) {
  const declares = isModality(category.modality);
  return pairFields
    .superRefine((data, ctx) => {
      if (!declares) return;
      requireDeclaration(ctx, [], data.gender, data.level);
      requireDeclaration(ctx, ['partner'], data.partner.gender, data.partner.level);
      const issues = pairIssues(
        category,
        toDeclared(data.gender, data.level),
        toDeclared(data.partner.gender, data.partner.level),
      );
      for (const issue of issues) {
        ctx.addIssue({ code: 'custom', path: issue.field.split('.'), message: issue.message });
      }
    })
    .refine((data) => data.phone !== data.partner.phone, {
      path: ['partner', 'phone'],
      message: BOOKING_MESSAGES.tournaments.partnerPhoneSame,
    });
}
