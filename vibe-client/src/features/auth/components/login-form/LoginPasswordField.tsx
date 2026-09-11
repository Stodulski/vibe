import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Eye, EyeOff, Lock } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { LoginDto } from '../../schemas/auth.schemas';

const t = ES_AR;

interface LoginPasswordFieldProps {
  register: UseFormRegister<LoginDto>;
  errors: FieldErrors<LoginDto>;
  showPassword: boolean;
  onToggleShowPassword: () => void;
}

export function LoginPasswordField({ register, errors, showPassword, onToggleShowPassword }: LoginPasswordFieldProps) {
  return (
    <FormField
      className="auth-stagger-2 space-y-1.5"
      label={t.auth.password}
      htmlFor="password"
      icon={Lock}
      error={errors.password?.message}
    >
      <Input
        id="password"
        type={showPassword ? 'text' : 'password'}
        autoComplete="current-password"
        aria-invalid={!!errors.password}
        aria-describedby={errors.password ? 'password-error' : undefined}
        className="h-11 pl-9 pr-10 text-sm sm:h-10"
        {...register('password')}
      />
      <button
        type="button"
        onClick={onToggleShowPassword}
        className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-lg p-2 text-text-tertiary transition-colors hover:text-text-secondary"
        aria-label={showPassword ? t.auth.hidePassword : t.auth.showPassword}
      >
        {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
      </button>
    </FormField>
  );
}
