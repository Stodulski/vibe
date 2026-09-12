import type { Control, FieldErrors, UseFormRegister } from 'react-hook-form';
import { Controller } from 'react-hook-form';
import { User } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { PhoneInput } from '@/shared/components/common/PhoneInput';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { RegisterDto } from '../../schemas/auth.schema';

const t = ES_AR;

interface RegisterContactFieldsProps {
  register: UseFormRegister<RegisterDto>;
  control: Control<RegisterDto>;
  errors: FieldErrors<RegisterDto>;
}

export function RegisterContactFields({ register, control, errors }: RegisterContactFieldsProps) {
  return (
    <>
      <div className="auth-stagger-1 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <FormField label={t.auth.firstName} htmlFor="first_name" icon={User} error={errors.first_name?.message}>
          <Input
            id="first_name"
            placeholder={t.placeholders.firstName}
            autoComplete="given-name"
            aria-invalid={!!errors.first_name}
            aria-describedby={errors.first_name ? 'first_name-error' : undefined}
            className="h-11 pl-9 text-sm sm:h-10"
            {...register('first_name')}
          />
        </FormField>
        <FormField label={t.auth.lastName} htmlFor="last_name" icon={User} error={errors.last_name?.message}>
          <Input
            id="last_name"
            placeholder={t.placeholders.lastName}
            autoComplete="family-name"
            aria-invalid={!!errors.last_name}
            aria-describedby={errors.last_name ? 'last_name-error' : undefined}
            className="h-11 pl-9 text-sm sm:h-10"
            {...register('last_name')}
          />
        </FormField>
      </div>

      <FormField
        className="auth-stagger-2 space-y-1.5"
        label={t.auth.phone}
        htmlFor="phone"
        error={errors.phone?.message}
      >
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
    </>
  );
}
