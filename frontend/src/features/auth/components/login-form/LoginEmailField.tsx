import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Mail } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { LoginDto } from '../../schemas/auth.schemas';

const t = ES_AR;

interface LoginEmailFieldProps {
  register: UseFormRegister<LoginDto>;
  errors: FieldErrors<LoginDto>;
}

export function LoginEmailField({ register, errors }: LoginEmailFieldProps) {
  return (
    <FormField
      className="auth-stagger-1 space-y-1.5"
      label={t.auth.email}
      htmlFor="email"
      icon={Mail}
      error={errors.email?.message}
    >
      <Input
        id="email"
        type="email"
        placeholder={t.placeholders.email}
        autoComplete="email"
        aria-invalid={!!errors.email}
        aria-describedby={errors.email ? 'email-error' : undefined}
        className="h-11 pl-9 text-sm sm:h-10"
        {...register('email')}
      />
    </FormField>
  );
}
