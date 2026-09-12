import type { Ref } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from 'react-router-dom';
import { Loader2, Mail } from 'lucide-react';
import { forgotPasswordSchema, type ForgotPasswordDto } from '@/features/auth';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { TurnstileField, type TurnstileFieldHandle } from '@/shared/components/common/TurnstileField';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';

const t = ES_AR;

interface ForgotPasswordFormProps {
  loading: boolean;
  /** `loading`, plus a Turnstile challenge configured but not yet solved — computed by `ForgotPasswordPage`. */
  submitDisabled: boolean;
  onSubmit: (data: ForgotPasswordDto) => void;
  turnstileRef: Ref<TurnstileFieldHandle>;
  onTurnstileTokenChange: (token: string | undefined) => void;
}

export function ForgotPasswordForm({
  loading,
  submitDisabled,
  onSubmit,
  turnstileRef,
  onTurnstileTokenChange,
}: ForgotPasswordFormProps) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ForgotPasswordDto>({
    resolver: zodResolver(forgotPasswordSchema),
  });

  return (
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
      <FormField label={t.auth.email} htmlFor="email" icon={Mail} error={errors.email?.message}>
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

      <TurnstileField ref={turnstileRef} onTokenChange={onTurnstileTokenChange} />

      <Button
        type="submit"
        className="h-11 w-full rounded-full font-semibold transition-colors hover:brightness-110"
        disabled={submitDisabled}
      >
        {loading ? (
          <>
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            {t.auth.forgotPasswordSending}
          </>
        ) : (
          t.auth.forgotPasswordSend
        )}
      </Button>

      <p className="!mt-4 text-center text-sm text-text-tertiary">
        <Link
          to="/login"
          className="font-medium text-primary-400 underline-offset-4 transition-colors hover:text-primary-300 hover:underline"
        >
          {t.auth.goToLogin}
        </Link>
      </p>
    </form>
  );
}
