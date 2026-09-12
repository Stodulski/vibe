import { useRef } from 'react';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useTurnstileChallenge } from '@/shared/hooks/useTurnstileChallenge';
import type { TurnstileFieldHandle } from '@/shared/components/common/TurnstileField';
import { ForgotPasswordForm } from './forgot-password/ForgotPasswordForm';
import { ForgotPasswordSuccess } from './forgot-password/ForgotPasswordSuccess';
import { useForgotPassword } from '@/features/auth/hooks/useForgotPassword';

const t = ES_AR;

export default function ForgotPasswordPage() {
  usePageTitle(t.auth.forgotPasswordTitle);
  const turnstileRef = useRef<TurnstileFieldHandle>(null);
  const turnstile = useTurnstileChallenge();
  const { mutate, isPending, sent } = useForgotPassword({
    resetTurnstile: () => turnstileRef.current?.reset(),
  });

  return (
    <AuthSplitLayout>
      <div className="auth-card w-full">
        <div className="flex flex-col items-center px-0 pt-2 pb-1 md:pt-6">
          <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
            {t.auth.forgotPasswordTitle}
          </h1>
          <p className="text-text-tertiary mt-1 text-center text-sm">{t.auth.forgotPasswordDescription}</p>
        </div>

        <div className="px-0 pt-4 pb-6">
          {sent ? (
            <ForgotPasswordSuccess />
          ) : (
            <ForgotPasswordForm
              loading={isPending}
              submitDisabled={turnstile.isBlocked(isPending)}
              turnstileRef={turnstileRef}
              onTurnstileTokenChange={turnstile.onTokenChange}
              onSubmit={(data) => {
                mutate({ ...data, ...turnstile.payload() });
              }}
            />
          )}
        </div>
      </div>
    </AuthSplitLayout>
  );
}
