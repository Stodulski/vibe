import { Link } from 'react-router-dom';
import { CheckCircle2, XCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { MPCallbackErrorReason, MPCallbackStatus as Status } from './useMPCallback';

const t = ES_AR;

const ERROR_COPY: Record<MPCallbackErrorReason, string> = {
  conflict: t.mp.connectErrorConflict,
  expired: t.mp.connectErrorExpired,
  failed: t.mp.connectError,
};

export function MPCallbackStatus({
  status,
  errorReason,
  returnPath,
}: {
  status: Status;
  errorReason: MPCallbackErrorReason;
  returnPath: string;
}) {
  return (
    <div className="space-y-4 px-4 text-center">
      {status === 'processing' && (
        <>
          <LoadingSpinner size="lg" />
          <p className="text-text-secondary text-sm">{t.mp.connecting}</p>
        </>
      )}
      {status === 'success' && (
        <>
          <CheckCircle2 className="text-success-text mx-auto size-10" />
          <p className="text-text-primary text-sm font-medium">{t.mp.connectSuccess}</p>
        </>
      )}
      {status === 'error' && (
        <>
          <XCircle className="text-error-text mx-auto size-10" />
          <p role="alert" className="text-text-primary mx-auto max-w-sm text-sm font-medium">
            {ERROR_COPY[errorReason]}
          </p>
          {/* The authorization code is single-use, so "retry" means starting the
              connection over: back to the page that holds the connect action. A
              conflict is not fixed by starting over, so it only offers the way back. */}
          <Button asChild variant={errorReason === 'conflict' ? 'outline' : 'default'}>
            <Link to={returnPath} replace>
              {errorReason === 'conflict' ? t.common.back : t.mp.connectRetry}
            </Link>
          </Button>
        </>
      )}
    </div>
  );
}
