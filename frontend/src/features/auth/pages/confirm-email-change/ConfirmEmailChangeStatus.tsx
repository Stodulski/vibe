import { Link } from 'react-router-dom';
import { CheckCircle2, XCircle, Loader2, Mail, AlertTriangle } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { ConfirmEmailChangeStatus as Status } from '@/features/auth/hooks/useConfirmEmailChange';

const t = ES_AR;

const confirmButtonClass =
  'bg-primary text-primary-foreground focus-visible:ring-ring/50 mt-6 inline-flex h-11 items-center justify-center rounded-full px-8 text-sm font-semibold transition-colors hover:brightness-110 focus-visible:ring-[3px]';

interface ConfirmEmailChangeStatusProps {
  status: Status;
  onConfirm: () => void;
}

/**
 * 'idle' (waiting for the first click) and the two retryable failures
 * ('rate_limited', 'error') share this screen — a confirm/retry button under
 * an explanatory message — and differ only in icon tone and copy. Split out
 * of `ConfirmEmailChangeStatus` to keep that switch itself short.
 */
function ConfirmOrRetry({ status, onConfirm }: { status: 'idle' | 'rate_limited' | 'error'; onConfirm: () => void }) {
  const message =
    status === 'rate_limited'
      ? t.auth.confirmEmailChangeRateLimited
      : status === 'error'
        ? t.auth.confirmEmailChangeGenericError
        : t.auth.confirmEmailChangeExplain;
  const iconWrapClass =
    status === 'idle'
      ? 'bg-info-bg ring-info-border mb-4 rounded-2xl p-3 ring-1'
      : 'bg-warning-bg ring-warning-border mb-4 rounded-2xl p-3 ring-1';

  return (
    <>
      <div className={iconWrapClass}>
        {status === 'idle' ? (
          <Mail className="text-info-icon size-8" />
        ) : (
          <AlertTriangle className="text-warning-icon size-8" />
        )}
      </div>
      <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
        {t.auth.confirmEmailChangeConfirmTitle}
      </h1>
      <p className="text-text-tertiary mt-2 text-center text-sm">{message}</p>
      <button type="button" onClick={onConfirm} className={confirmButtonClass}>
        {status === 'idle' ? t.auth.confirmEmailChangeConfirmButton : t.auth.confirmEmailChangeRetryButton}
      </button>
    </>
  );
}

export function ConfirmEmailChangeStatus({ status, onConfirm }: ConfirmEmailChangeStatusProps) {
  if (status === 'loading') {
    return (
      <>
        <Loader2 className="text-primary-400 mb-4 size-10 animate-spin" />
        <p className="text-text-tertiary text-sm">{t.auth.confirmEmailChangeVerifying}</p>
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
          {t.auth.confirmEmailChangeSuccess}
        </h1>
        <p className="text-text-tertiary mt-2 text-center text-sm">{t.auth.confirmEmailChangeSuccessDesc}</p>
        <Link to="/login" replace className={confirmButtonClass}>
          {t.auth.goToLogin}
        </Link>
      </>
    );
  }

  if (status === 'idle' || status === 'rate_limited' || status === 'error') {
    return <ConfirmOrRetry status={status} onConfirm={onConfirm} />;
  }

  // 'invalid' | 'taken' — nothing left to retry, only a way back to login.
  return (
    <>
      <div className="bg-error-bg ring-error-border mb-4 rounded-2xl p-3 ring-1">
        <XCircle className="text-error-icon size-8" />
      </div>
      <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
        {t.layout.errorTitle}
      </h1>
      <p className="text-text-tertiary mt-2 text-center text-sm">
        {status === 'taken' ? t.auth.confirmEmailChangeTaken : t.auth.confirmEmailChangeExpired}
      </p>
      <Link to="/login" className={confirmButtonClass}>
        {t.auth.goToLogin}
      </Link>
    </>
  );
}
