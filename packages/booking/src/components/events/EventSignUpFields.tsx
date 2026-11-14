import { Controller, type Control, type FieldErrors, type UseFormRegister } from 'react-hook-form';
import { FieldRequirement, FormField, Input, PhoneInput } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { EventSignUpFormData } from '../../schemas/event-signup.schema';

const t = BOOKING_MESSAGES;

interface EventSignUpFieldsProps {
  register: UseFormRegister<EventSignUpFormData>;
  control: Control<EventSignUpFormData>;
  errors: FieldErrors<EventSignUpFormData>;
}

type Props = EventSignUpFieldsProps;

function NameFields({ register, errors }: Pick<Props, 'register' | 'errors'>) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField
        label={
          <>
            {t.publicBooking.firstName}
            <FieldRequirement required />
          </>
        }
        htmlFor="event_first_name"
        error={errors.first_name?.message}
      >
        <Input
          id="event_first_name"
          autoComplete="given-name"
          placeholder={t.placeholders.firstName}
          className="h-12 text-base"
          {...register('first_name')}
        />
      </FormField>
      <FormField
        label={
          <>
            {t.publicBooking.lastName}
            <FieldRequirement required />
          </>
        }
        htmlFor="event_last_name"
        error={errors.last_name?.message}
      >
        <Input
          id="event_last_name"
          autoComplete="family-name"
          placeholder={t.placeholders.lastName}
          className="h-12 text-base"
          {...register('last_name')}
        />
      </FormField>
    </div>
  );
}

function PhoneField({ control, errors }: Pick<Props, 'control' | 'errors'>) {
  return (
    <FormField
      label={
        <>
          {t.publicBooking.phone}
          <FieldRequirement required />
        </>
      }
      htmlFor="event_phone"
      helpText={t.publicBooking.phoneHelper}
      error={errors.phone?.message}
    >
      <Controller
        name="phone"
        control={control}
        render={({ field }) => (
          <PhoneInput
            id="event_phone"
            value={field.value}
            onChange={field.onChange}
            onBlur={field.onBlur}
            placeholder="11 2345 6789"
            inputClassName="h-12 text-base"
            aria-invalid={!!errors.phone}
          />
        )}
      />
    </FormField>
  );
}

function EmailField({ register, errors }: Pick<Props, 'register' | 'errors'>) {
  return (
    <FormField
      label={
        <>
          {t.publicBooking.email}
          <FieldRequirement required={false} />
        </>
      }
      htmlFor="event_email"
      error={errors.email?.message}
    >
      <Input
        id="event_email"
        type="email"
        inputMode="email"
        autoComplete="email"
        placeholder={t.publicBooking.emailPlaceholder}
        className="h-12 text-base"
        {...register('email')}
      />
    </FormField>
  );
}

/** Name and phone required, email optional. The same chrome as the booking form. */
export function EventSignUpFields({ register, control, errors }: Props) {
  return (
    <>
      <NameFields register={register} errors={errors} />
      <PhoneField control={control} errors={errors} />
      <EmailField register={register} errors={errors} />
    </>
  );
}
