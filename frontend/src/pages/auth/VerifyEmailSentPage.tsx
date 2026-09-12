import { Link, useLocation } from 'react-router-dom';
import { Mail } from 'lucide-react';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { verifyEmailSentStateSchema } from '@/features/auth';
import { ResendVerificationButton } from './verify-email-sent/ResendVerificationButton';
import { useResendVerification } from './verify-email-sent/useResendVerification';

const t = ES_AR;

export default function VerifyEmailSentPage() {
  usePageTitle(t.auth.verifyEmailSent);
  const location = useLocation();
  // `location.state` isn't guaranteed to match this shape (see
  // 06-auth-shared-tooling.md M9) — safeParse instead of trusting an `as`
  // cast, so a malformed `email` (e.g. not a string) falls back to "no
  // email" rather than being passed on to the resend request as-is.
  const parsedState = verifyEmailSentStateSchema.safeParse(location.state);
  const email = parsedState.success ? parsedState.data.email : undefined;
  const { resending, cooldown, handleResend } = useResendVerification(email);

  return (
    <AuthSplitLayout>
      <div className="auth-card w-full">
        <div className="flex flex-col items-center px-0 pt-2 pb-2 text-center md:pt-6">
          <div className="animate-pulse-glow bg-primary-500/10 ring-primary-500/25 mb-4 rounded-2xl p-3 ring-1">
            <Mail className="text-primary-400 size-7" />
          </div>
          <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
            {t.auth.verifyEmailSent}
          </h1>
          <p className="text-text-tertiary mt-2 text-sm leading-relaxed">{t.auth.verifyEmailSentDesc}</p>
          {email && <p className="text-text-secondary mt-1 text-sm font-medium">{email}</p>}
        </div>

        <div className="px-0 pt-4 pb-6">
          {email && <ResendVerificationButton resending={resending} cooldown={cooldown} onResend={handleResend} />}
          <Link
            to="/login"
            className="text-primary-400 hover:text-primary-300 block text-center text-sm font-medium underline-offset-4 transition-colors hover:underline"
          >
            {t.auth.goToLogin}
          </Link>
        </div>
      </div>
    </AuthSplitLayout>
  );
}
