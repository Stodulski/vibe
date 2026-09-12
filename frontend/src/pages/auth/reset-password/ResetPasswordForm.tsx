import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from 'react-router-dom';
import { Loader2, Lock, Eye, EyeOff } from 'lucide-react';
import { resetPasswordSchema, type ResetPasswordDto } from '@/features/auth';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';

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
  } = useForm<ResetPasswordDto>({
    resolver: zodResolver(resetPasswordSchema),
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

      <Button
        type="submit"
        className="h-11 w-full rounded-full font-semibold transition-colors hover:brightness-110"
        disabled={loading}
      >
        {loading ? (
          <>
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            {t.auth.resetPasswordSubmitting}
          </>
        ) : (
          t.auth.resetPasswordSubmit
        )}
      </Button>

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
