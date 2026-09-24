import { useState } from 'react';

import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from 'react-router-dom';
import { Lock, Eye, EyeOff } from 'lucide-react';
import { resetPasswordSchema, type ResetPasswordDto } from '@/features/auth';
import { LoadingButton } from '@/shared/components/common/LoadingButton';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useAppForm, submitHandler } from '@/shared/lib/form';

const t = ES_AR;

interface ResetPasswordFormProps {
  loading: boolean;
  onSubmit: (data: ResetPasswordDto) => void;
}

export function ResetPasswordForm({ loading, onSubmit }: ResetPasswordFormProps) {
  const [showPassword, setShowPassword] = useState(false);
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useAppForm<ResetPasswordDto>({
    resolver: zodResolver(resetPasswordSchema),
    // FORM-09: without this the inputs mount with `value === undefined`, so
    // React treats them as uncontrolled and then switches them to controlled on
    // the first keystroke — a dev warning, and a `reset()` that cannot put the
    // field back to a value it never had.
    defaultValues: { password: '' },
  });

  return (
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="w-full space-y-4" noValidate>
      <FormField label={t.auth.newPassword} htmlFor="password" icon={Lock} error={errors.password?.message}>
        <Input
          id="password"
          type={showPassword ? 'text' : 'password'}
          autoComplete="new-password"
          aria-invalid={!!errors.password}
          aria-describedby={errors.password ? 'password-error' : undefined}
          className="h-11 pr-10 pl-9 text-sm sm:h-10"
          {...register('password')}
        />
        <button
          type="button"
          onClick={() => {
            setShowPassword(!showPassword);
          }}
          className="text-text-tertiary hover:text-text-secondary absolute top-1/2 right-1.5 -translate-y-1/2 rounded-lg p-2 transition-colors"
          aria-label={showPassword ? t.auth.hidePassword : t.auth.showPassword}
        >
          {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </button>
      </FormField>

      <LoadingButton
        type="submit"
        className="h-11 w-full rounded-full font-semibold transition-colors hover:brightness-110"
        loading={loading}
        loadingText={t.auth.resetPasswordSubmitting}
      >
        {t.auth.resetPasswordSubmit}
      </LoadingButton>

      <p className="text-text-tertiary !mt-4 text-center text-sm">
        <Link
          to="/login"
          className="text-primary-400 hover:text-primary-300 font-medium underline-offset-4 transition-colors hover:underline"
        >
          {t.auth.goToLogin}
        </Link>
      </p>
    </form>
  );
}
