import { Link } from 'react-router-dom';
import { CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface VerifyEmailStatusProps {
  status: 'loading' | 'success' | 'error';
  countdown: number;
}

export function VerifyEmailStatus({ status, countdown }: VerifyEmailStatusProps) {
  if (status === 'loading') {
    return (
      <>
        <Loader2 className="text-primary-400 mb-4 size-10 animate-spin" />
        <p className="text-text-tertiary text-sm">{t.auth.verifying}</p>
      </>
    );
  }

  if (status === 'success') {
    return (
      <>
        <div className="bg-success-bg ring-success-border mb-4 rounded-2xl p-3 ring-1">
          <CheckCircle2 className="text-success-icon size-8" />
        </div>
        <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
          {t.auth.verifyEmailSuccess}
        </h1>
        <p className="text-text-tertiary mt-2 text-center text-sm">{t.auth.verifyEmailSuccessDesc}</p>
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
      <p className="text-text-tertiary mt-2 text-center text-sm">{t.auth.verifyEmailError}</p>
      <div className="mt-6 flex flex-col items-center gap-3">
        <Link
          to="/login"
          className="bg-primary text-primary-foreground focus-visible:ring-ring/50 inline-flex h-11 items-center justify-center rounded-full px-8 text-sm font-semibold transition-colors hover:brightness-110 focus-visible:ring-[3px]"
        >
          {t.auth.goToLogin}
        </Link>
        <Link
          to="/register"
          className="text-primary-400 hover:text-primary-300 text-sm font-medium underline-offset-4 transition-colors hover:underline"
        >
          {t.auth.registerHere}
        </Link>
      </div>
    </>
  );
}
