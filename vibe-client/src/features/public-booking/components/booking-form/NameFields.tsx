import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { PublicBookingFormData } from '../../schemas/public-booking.schemas';

const t = ES_AR;

interface NameFieldsProps {
  register: UseFormRegister<PublicBookingFormData>;
  errors: FieldErrors<PublicBookingFormData>;
}

/**
 * These fields carried their own hand-written label and error markup, which
 * is how they ended up showing errors below the input while the rest of the
 * app moved them above. They use the shared chrome now.
 */
export function NameFields({ register, errors }: NameFieldsProps) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField
        label={
          <>
            {t.publicBooking.firstName}
            <FieldRequirement required />
          </>
        }
        htmlFor="client_first_name"
        error={errors.client_first_name?.message}
      >
        <Input
          id="client_first_name"
          autoComplete="given-name"
          placeholder={t.placeholders.firstName}
          className="h-12 text-base"
          aria-invalid={!!errors.client_first_name}
          aria-describedby={errors.client_first_name ? 'client_first_name-error' : undefined}
          {...register('client_first_name')}
        />
      </FormField>
      <FormField
        label={
          <>
            {t.publicBooking.lastName}
            <FieldRequirement required />
          </>
        }
        htmlFor="client_last_name"
        error={errors.client_last_name?.message}
      >
        <Input
          id="client_last_name"
          autoComplete="family-name"
          placeholder={t.placeholders.lastName}
          className="h-12 text-base"
          aria-invalid={!!errors.client_last_name}
          aria-describedby={errors.client_last_name ? 'client_last_name-error' : undefined}
          {...register('client_last_name')}
        />
      </FormField>
    </div>
  );
}
