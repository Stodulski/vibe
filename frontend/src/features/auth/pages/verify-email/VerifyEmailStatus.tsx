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
        <Loader2 className="mb-4 size-10 animate-spin text-primary-400" />
        <p className="text-sm text-text-tertiary">{t.auth.verifying}</p>
      </>
    );
  }

  if (status === 'success') {
    return (
      <>
        <div className="mb-4 rounded-2xl bg-success-bg p-3 ring-1 ring-success-border">
          <CheckCircle2 className="size-8 text-success-icon" />
        </div>
        <h1 className="font-display text-xl font-bold tracking-tight text-text-primary sm:text-2xl">
          {t.auth.verifyEmailSuccess}
        </h1>
        <p className="mt-2 text-center text-sm text-text-tertiary">{t.auth.verifyEmailSuccessDesc}</p>
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
      <p className="mt-2 text-center text-sm text-text-tertiary">{t.auth.verifyEmailError}</p>
      <div className="mt-6 flex flex-col items-center gap-3">
        <Link
          to="/login"
          className="inline-flex h-11 items-center justify-center rounded-full bg-primary px-8 text-sm font-semibold text-primary-foreground transition-colors hover:brightness-110 focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {t.auth.goToLogin}
        </Link>
        <Link
          to="/register"
          className="text-sm font-medium text-primary-400 underline-offset-4 transition-colors hover:text-primary-300 hover:underline"
        >
          {t.auth.registerHere}
        </Link>
      </div>
    </>
  );
}
