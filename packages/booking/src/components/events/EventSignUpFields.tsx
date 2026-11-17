import type { Control, FieldErrors, UseFormRegister } from 'react-hook-form';
import type { PublicOpenPlayEvent } from '../../api/types';
import { PersonFields } from '../signup/PersonFields';
import { PlayerDeclarationFields } from '../signup/PlayerDeclarationFields';
import type { EventSignUpFormData } from '../../schemas/event-signup.schema';
import { openPlayDeclaresLevel } from '../../lib/declaredPlayers';
import { isModality } from '../../lib/traitLabels';

interface EventSignUpFieldsProps {
  register: UseFormRegister<EventSignUpFormData>;
  control: Control<EventSignUpFormData>;
  errors: FieldErrors<EventSignUpFormData>;
  /** The event: its modality asks the person's gender, its range of levels asks their level, and its sport names both. */
  event: Pick<PublicOpenPlayEvent, 'sport' | 'modality' | 'level_min' | 'level_max'>;
}

/**
 * Name and phone required, email optional, the same chrome as the booking form.
 * An event with a modality adds the person's gender; one with a range adds the
 * person's level.
 */
export function EventSignUpFields({ register, control, errors, event }: EventSignUpFieldsProps) {
  const asksGender = isModality(event.modality);
  const asksLevel = openPlayDeclaresLevel(event);
  return (
    <>
      <PersonFields register={register} control={control} errors={errors} idPrefix="event_" withEmail />
      {(asksGender || asksLevel) && (
        <PlayerDeclarationFields
          register={register}
          errors={errors}
          idPrefix="event_"
          sport={event.sport}
          asksGender={asksGender}
          asksLevel={asksLevel}
        />
      )}
    </>
  );
}
