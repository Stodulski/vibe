import type { Ref } from 'react';

import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from 'react-router-dom';
import { Mail } from 'lucide-react';
import { forgotPasswordSchema, type ForgotPasswordDto } from '@/features/auth';
import { LoadingButton } from '@/shared/components/common/LoadingButton';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { TurnstileField, type TurnstileFieldHandle } from '@/shared/components/common/TurnstileField';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useAppForm, submitHandler } from '@/shared/lib/form';

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
  } = useAppForm<ForgotPasswordDto>({
    resolver: zodResolver(forgotPasswordSchema),
    // FORM-09: without this the inputs mount with `value === undefined`, so
    // React treats them as uncontrolled and then switches them to controlled on
    // the first keystroke — a dev warning, and a `reset()` that cannot put the
    // field back to a value it never had.
    defaultValues: { email: '' },
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

      <LoadingButton
        type="submit"
        className="h-11 w-full rounded-full font-semibold transition-colors hover:brightness-110"
        disabled={submitDisabled}
        loading={loading}
        loadingText={t.auth.forgotPasswordSending}
      >
        {t.auth.forgotPasswordSend}
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
