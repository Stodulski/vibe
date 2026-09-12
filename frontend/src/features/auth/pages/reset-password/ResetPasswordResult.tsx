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
        <div className="bg-success-bg ring-success-border mb-4 rounded-2xl p-3 ring-1">
          <CheckCircle2 className="text-success-icon size-8" />
        </div>
        <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
          {t.auth.resetPasswordTitle}
        </h1>
        <p className="text-text-tertiary mt-2 text-center text-sm">{t.auth.resetPasswordSuccess}</p>
        <Link
          to="/login"
          replace
          className="bg-primary text-primary-foreground focus-visible:ring-ring/50 mt-6 inline-flex h-11 items-center justify-center rounded-full px-8 text-sm font-semibold transition-colors hover:brightness-110 focus-visible:ring-[3px]"
        >
          {t.auth.goToLogin} ({countdown})
        </Link>
      </>
    );
  }

  return (
    <>
      <div className="bg-error-bg ring-error-border mb-4 rounded-2xl p-3 ring-1">
        <XCircle className="text-error-icon size-8" />
      </div>
      <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
        {t.layout.errorTitle}
      </h1>
      <p className="text-text-tertiary mt-2 text-center text-sm">{t.auth.resetPasswordError}</p>
      <div className="mt-6 flex flex-col items-center gap-3">
        <Link
          to="/forgot-password"
          className="bg-primary text-primary-foreground focus-visible:ring-ring/50 inline-flex h-11 items-center justify-center rounded-full px-8 text-sm font-semibold transition-colors hover:brightness-110 focus-visible:ring-[3px]"
        >
          {t.auth.forgotPasswordSend}
        </Link>
        <Link
          to="/login"
          className="text-primary-400 hover:text-primary-300 text-sm font-medium underline-offset-4 transition-colors hover:underline"
        >
          {t.auth.goToLogin}
        </Link>
      </div>
    </>
  );
}
