import { Link } from 'react-router-dom';
import { CheckCircle2, XCircle } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ResetPasswordResultProps {
  status: 'success' | 'error';
  countdown: number;
}

export function ResetPasswordResult({ status, countdown }: ResetPasswordResultProps) {
  if (status === 'success') {
    return (
      <>
        <div className="mb-4 rounded-2xl bg-success-bg p-3 ring-1 ring-success-border">
          <CheckCircle2 className="size-8 text-success-icon" />
        </div>
        <h1 className="font-display text-xl font-bold tracking-tight text-text-primary sm:text-2xl">
          {t.auth.resetPasswordTitle}
        </h1>
        <p className="mt-2 text-center text-sm text-text-tertiary">{t.auth.resetPasswordSuccess}</p>
        <Link
          to="/login"
          replace
          className="mt-6 inline-flex h-11 items-center justify-center rounded-full bg-primary px-8 text-sm font-semibold text-primary-foreground transition-colors hover:brightness-110 focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {t.auth.goToLogin} ({countdown})
        </Link>
      </>
    );
  }

  return (
    <>
      <div className="mb-4 rounded-2xl bg-error-bg p-3 ring-1 ring-error-border">
        <XCircle className="size-8 text-error-icon" />
      </div>
      <h1 className="font-display text-xl font-bold tracking-tight text-text-primary sm:text-2xl">
        {t.layout.errorTitle}
      </h1>
      <p className="mt-2 text-center text-sm text-text-tertiary">{t.auth.resetPasswordError}</p>
      <div className="mt-6 flex flex-col items-center gap-3">
        <Link
          to="/forgot-password"
          className="inline-flex h-11 items-center justify-center rounded-full bg-primary px-8 text-sm font-semibold text-primary-foreground transition-colors hover:brightness-110 focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {t.auth.forgotPasswordSend}
        </Link>
        <Link
          to="/login"
          className="text-sm font-medium text-primary-400 underline-offset-4 transition-colors hover:text-primary-300 hover:underline"
        >
          {t.auth.goToLogin}
        </Link>
      </div>
    </>
  );
}
