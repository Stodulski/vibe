import { Controller } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { PhoneInput } from '@/shared/components/common/PhoneInput';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Control, FieldErrors, UseFormRegister } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

interface ClientInfoFieldsProps {
  register: UseFormRegister<CreateBookingDto>;
  control: Control<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}

function ClientNameFields({ register, errors }: Omit<ClientInfoFieldsProps, 'control'>) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField
        label={
          <>
            {t.bookings.clientFirstName}
            <FieldRequirement required />
          </>
        }
        htmlFor="client-first-name"
        error={errors.client_first_name?.message}
      >
        <Input id="client-first-name" placeholder={t.placeholders.firstName} {...register('client_first_name')} />
      </FormField>
      <FormField
        label={
          <>
            {t.bookings.clientLastName}
            <FieldRequirement required />
          </>
        }
        htmlFor="client-last-name"
        error={errors.client_last_name?.message}
      >
        <Input id="client-last-name" placeholder={t.placeholders.lastName} {...register('client_last_name')} />
      </FormField>
    </div>
  );
}

function ClientPhoneAndEmailFields({ register, control, errors }: ClientInfoFieldsProps) {
  return (
    <>
      <FormField
        label={
          <>
            {t.bookings.clientPhone}
            <FieldRequirement required />
          </>
        }
        htmlFor="client-phone"
        error={errors.client_phone?.message}
      >
        <Controller
          name="client_phone"
          control={control}
          render={({ field }) => (
            <PhoneInput
              id="client-phone"
              value={field.value}
              onChange={field.onChange}
              onBlur={field.onBlur}
              placeholder="11 2345 6789"
              aria-invalid={!!errors.client_phone}
            />
          )}
        />
      </FormField>
      <FormField
        label={
          <>
            {t.auth.email}
            <FieldRequirement required={false} />
          </>
        }
        htmlFor="client-email"
        error={errors.client_email?.message}
      >
        <Input id="client-email" type="email" {...register('client_email')} placeholder={t.placeholders.clientEmail} />
      </FormField>
    </>
  );
}

export function ClientInfoFields({ register, control, errors }: ClientInfoFieldsProps) {
  return (
    <div className="space-y-3">
      <Label className="text-micro font-semibold uppercase tracking-wider text-text-tertiary">
        {t.bookings.clientData}
      </Label>
      <ClientNameFields register={register} errors={errors} />
      <ClientPhoneAndEmailFields register={register} control={control} errors={errors} />
    </div>
  );
}
