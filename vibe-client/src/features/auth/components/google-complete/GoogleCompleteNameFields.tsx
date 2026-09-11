import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { User } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { GoogleCompleteDto } from '../../schemas/auth.schemas';

const t = ES_AR;

interface GoogleCompleteNameFieldsProps {
  register: UseFormRegister<GoogleCompleteDto>;
  errors: FieldErrors<GoogleCompleteDto>;
}

/** First/last name, prefilled from the Google profile by the caller's `defaultValues` and editable. */
export function GoogleCompleteNameFields({ register, errors }: GoogleCompleteNameFieldsProps) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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
  );
}
