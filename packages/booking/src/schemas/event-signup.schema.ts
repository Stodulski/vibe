import '../lib/zodConfig';
import { z } from 'zod';
import { BOOKING_MESSAGES } from '../messages';
import { optionalEmailField, phoneField, firstNameField, lastNameField } from '../lib/validations';
import { openPlayDeclaresLevel, openPlayIssues, toDeclared, type EventTraits } from '../lib/declaredPlayers';
import { declarationWords, isModality } from '../lib/traitLabels';

const d = BOOKING_MESSAGES.declaration;

/**
 * The sign-up form: name and phone required, email optional (without it no
 * confirmation email is sent). The person's gender and level are strings until
 * they are chosen; they are asked only when the event has a modality (gender)
 * or a range of levels (level).
 */
const eventFields = z.object({
  first_name: firstNameField,
  last_name: lastNameField,
  phone: phoneField,
  email: optionalEmailField,
  gender: z.string(),
  level: z.string(),
});

export type EventSignUpFormData = z.infer<typeof eventFields>;

/**
 * The sign-up form for one open-play event. An event with a modality needs the
 * person's gender, and one with a range needs their level; both must fit the
 * event. An event with neither takes anyone.
 */
export function eventSignUpSchema(event: EventTraits) {
  const askGender = isModality(event.modality);
  const askLevel = openPlayDeclaresLevel(event);
  const words = declarationWords(event.sport);
  return eventFields.superRefine((data, ctx) => {
    if (!askGender && !askLevel) return;
    if (askGender && data.gender !== 'men' && data.gender !== 'women') {
      ctx.addIssue({ code: 'custom', path: ['gender'], message: d.genderRequired(words.genderOr) });
    }
    if (askLevel && !/^[1-9]$/.test(data.level)) {
      ctx.addIssue({ code: 'custom', path: ['level'], message: d.levelRequired });
    }
    for (const issue of openPlayIssues(event, toDeclared(data.gender, data.level))) {
      ctx.addIssue({ code: 'custom', path: [issue.field], message: issue.message });
    }
  });
}
