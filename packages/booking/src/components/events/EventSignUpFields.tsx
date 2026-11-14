import type { Control, FieldErrors, UseFormRegister } from 'react-hook-form';
import { PersonFields } from '../signup/PersonFields';
import type { EventSignUpFormData } from '../../schemas/event-signup.schema';

interface EventSignUpFieldsProps {
  register: UseFormRegister<EventSignUpFormData>;
  control: Control<EventSignUpFormData>;
  errors: FieldErrors<EventSignUpFormData>;
}

/** Name and phone required, email optional. The same chrome as the booking form. */
export function EventSignUpFields({ register, control, errors }: EventSignUpFieldsProps) {
  return <PersonFields register={register} control={control} errors={errors} idPrefix="event_" withEmail />;
}
