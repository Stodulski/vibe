import type { Control, FieldErrors, UseFormRegister } from 'react-hook-form';
import { PersonFields } from '../signup/PersonFields';
import { PlayerDeclarationFields } from '../signup/PlayerDeclarationFields';
import type { EventSignUpFormData } from '../../schemas/event-signup.schema';

interface EventSignUpFieldsProps {
  register: UseFormRegister<EventSignUpFormData>;
  control: Control<EventSignUpFormData>;
  errors: FieldErrors<EventSignUpFormData>;
  /** An event with a modality also asks the person's gender and level. */
  declares: boolean;
}

/**
 * Name and phone required, email optional, the same chrome as the booking form.
 * An event with a modality adds the person's gender and category.
 */
export function EventSignUpFields({ register, control, errors, declares }: EventSignUpFieldsProps) {
  return (
    <>
      <PersonFields register={register} control={control} errors={errors} idPrefix="event_" withEmail />
      {declares && <PlayerDeclarationFields register={register} errors={errors} idPrefix="event_" />}
    </>
  );
}
