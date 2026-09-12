import type { Control, FieldErrors } from 'react-hook-form';
import { Controller } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { PhoneInput } from '@/shared/components/common/PhoneInput';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { GoogleCompleteDto } from '../../schemas/auth.schema';

const t = ES_AR;

interface GoogleCompletePhoneFieldProps {
  control: Control<GoogleCompleteDto>;
  errors: FieldErrors<GoogleCompleteDto>;
}

/** Reuses the register form's own phone input and `+54` validation rules. */
export function GoogleCompletePhoneField({ control, errors }: GoogleCompletePhoneFieldProps) {
  return (
    <FormField label={t.auth.phone} htmlFor="phone" error={errors.phone?.message}>
      <Controller
        name="phone"
        control={control}
        render={({ field }) => (
          <PhoneInput
            id="phone"
            value={field.value}
            onChange={field.onChange}
            onBlur={field.onBlur}
            placeholder="11 2345 6789"
            inputClassName="h-11 text-sm sm:h-10"
            selectClassName="h-11 sm:h-10"
            aria-invalid={!!errors.phone}
            aria-describedby={errors.phone ? 'phone-error' : undefined}
          />
        )}
      />
    </FormField>
  );
}
