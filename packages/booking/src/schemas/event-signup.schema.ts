import '../lib/zodConfig';
import { z } from 'zod';
import type { PublicOpenPlayEvent } from '../api/types';
import { BOOKING_MESSAGES } from '../messages';
import { optionalEmailField, phoneField, firstNameField, lastNameField } from '../lib/validations';
import { openPlayIssues, toDeclared } from '../lib/declaredPlayers';
import { isModality } from '../lib/traitLabels';

const d = BOOKING_MESSAGES.declaration;

/**
 * The sign-up form: name and phone required, email optional (without it no
 * confirmation email is sent). The person's gender and level are strings until
 * they are chosen, and they are asked only for an event that has a modality.
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

type EventTraits = Pick<PublicOpenPlayEvent, 'modality' | 'level_min' | 'level_max'>;

/**
 * The sign-up form for one open-play event. An event with a modality needs the
 * person's gender and level, which must fit it. An event without one takes
 * anyone.
 */
export function eventSignUpSchema(event: EventTraits) {
  const declares = isModality(event.modality);
  return eventFields.superRefine((data, ctx) => {
    if (!declares) return;
    if (data.gender !== 'men' && data.gender !== 'women') {
      ctx.addIssue({ code: 'custom', path: ['gender'], message: d.genderRequired });
    }
    if (!/^[1-9]$/.test(data.level)) {
      ctx.addIssue({ code: 'custom', path: ['level'], message: d.levelRequired });
    }
    for (const issue of openPlayIssues(event, toDeclared(data.gender, data.level))) {
      ctx.addIssue({ code: 'custom', path: [issue.field], message: issue.message });
    }
  });
}
