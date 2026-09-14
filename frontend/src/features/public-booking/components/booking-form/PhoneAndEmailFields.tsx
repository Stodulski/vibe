import { Controller, type Control, type FieldErrors, type UseFormRegister } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { PhoneInput } from '@/shared/components/common/PhoneInput';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { PublicBookingFormData } from '../../schemas/public-booking.schema';

const t = ES_AR;

interface PhoneAndEmailFieldsProps {
  register: UseFormRegister<PublicBookingFormData>;
  control: Control<PublicBookingFormData>;
  errors: FieldErrors<PublicBookingFormData>;
}

export function PhoneAndEmailFields({ register, control, errors }: PhoneAndEmailFieldsProps) {
  return (
    <>
      <FormField
        label={
          <>
            {t.publicBooking.phone}
            <FieldRequirement required />
          </>
        }
        htmlFor="client_phone"
        helpText={t.publicBooking.phoneHelper}
        error={errors.client_phone?.message}
      >
        <Controller
          name="client_phone"
          control={control}
          render={({ field }) => (
            <PhoneInput
              id="client_phone"
              value={field.value}
              onChange={field.onChange}
              onBlur={field.onBlur}
              placeholder="11 2345 6789"
              inputClassName="h-12 text-base"
              aria-invalid={!!errors.client_phone}
              aria-describedby={errors.client_phone ? 'client_phone-error' : undefined}
            />
          )}
        />
      </FormField>

      <FormField
        label={
          <>
            {t.publicBooking.email}
            <FieldRequirement required={false} />
          </>
        }
        htmlFor="client_email"
        error={errors.client_email?.message}
      >
        <Input
          id="client_email"
          type="email"
          inputMode="email"
          autoComplete="email"
          placeholder={t.publicBooking.emailPlaceholder}
          className="h-12 text-base"
          aria-invalid={!!errors.client_email}
          aria-describedby={errors.client_email ? 'client_email-error' : undefined}
          {...register('client_email')}
        />
      </FormField>
    </>
  );
}
