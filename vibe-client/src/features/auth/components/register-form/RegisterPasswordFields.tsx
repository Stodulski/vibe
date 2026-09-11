import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Eye, EyeOff, Lock } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { RegisterDto } from '../../schemas/auth.schemas';

const t = ES_AR;

interface RegisterConfirmPasswordFieldProps {
  register: UseFormRegister<RegisterDto>;
  errors: FieldErrors<RegisterDto>;
  showPassword: boolean;
}

function RegisterConfirmPasswordField({ register, errors, showPassword }: RegisterConfirmPasswordFieldProps) {
  return (
    <FormField
      label={t.auth.confirmPassword}
      htmlFor="confirm_password"
      icon={Lock}
      error={errors.confirm_password?.message}
    >
      <Input
        id="confirm_password"
        type={showPassword ? 'text' : 'password'}
        autoComplete="new-password"
        aria-invalid={!!errors.confirm_password}
        aria-describedby={errors.confirm_password ? 'confirm_password-error' : undefined}
        className="h-11 pl-9 text-sm sm:h-10"
        {...register('confirm_password')}
      />
    </FormField>
  );
}

interface RegisterPasswordFieldsProps {
  register: UseFormRegister<RegisterDto>;
  errors: FieldErrors<RegisterDto>;
  showPassword: boolean;
  onToggleShowPassword: () => void;
}

export function RegisterPasswordFields({
  register,
  errors,
  showPassword,
  onToggleShowPassword,
}: RegisterPasswordFieldsProps) {
  return (
    <div className="auth-stagger-3 grid grid-cols-1 gap-3">
      <FormField label={t.auth.password} htmlFor="password" icon={Lock} error={errors.password?.message}>
        <Input
          id="password"
          type={showPassword ? 'text' : 'password'}
          autoComplete="new-password"
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
      <RegisterConfirmPasswordField register={register} errors={errors} showPassword={showPassword} />
    </div>
  );
}
