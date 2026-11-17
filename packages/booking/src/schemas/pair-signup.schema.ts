import '../lib/zodConfig';
import { z } from 'zod';
import { BOOKING_MESSAGES } from '../messages';
import { emailField, phoneField, firstNameField, lastNameField } from '../lib/validations';
import { pairDeclaresLevel, pairIssues, toDeclared, type PairCategory } from '../lib/declaredPlayers';
import { declarationWords, isModality, type DeclarationWords } from '../lib/traitLabels';

const d = BOOKING_MESSAGES.declaration;

/**
 * The fields of the pair sign-up form. The payer's name, phone and email are
 * required, and the partner's name and phone are required (the partner has no
 * email field). Each player's gender and level are strings until the person
 * chooses them.
 */
const pairFields = z.object({
  first_name: firstNameField,
  last_name: lastNameField,
  phone: phoneField,
  email: emailField,
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

/** Which declarations a category asks of each player: the gender when it has a modality, the level when it has a level or a Suma. */
interface Asked {
  gender: boolean;
  level: boolean;
}

function requireDeclaration(
  ctx: z.RefinementCtx,
  path: string[],
  gender: string,
  level: string,
  asked: Asked,
  words: DeclarationWords,
): void {
  if (asked.gender && gender !== 'men' && gender !== 'women') {
    ctx.addIssue({ code: 'custom', path: [...path, 'gender'], message: d.genderRequired(words.genderOr) });
  }
  if (asked.level && !/^[1-9]$/.test(level)) {
    ctx.addIssue({ code: 'custom', path: [...path, 'level'], message: d.levelRequired });
  }
}

/**
 * The pair sign-up form for one category. A category with a modality asks each
 * player's gender, and one with a level or a Suma asks their level as well; the
 * answers must fit the category's rules (the same rules the server checks). A
 * category without either takes the pair as it is.
 */
export function pairSignUpSchema(category: PairCategory) {
  const asked: Asked = { gender: isModality(category.modality), level: pairDeclaresLevel(category) };
  const words = declarationWords(category.sport);
  return pairFields
    .superRefine((data, ctx) => {
      if (!asked.gender && !asked.level) return;
      requireDeclaration(ctx, [], data.gender, data.level, asked, words);
      requireDeclaration(ctx, ['partner'], data.partner.gender, data.partner.level, asked, words);
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
