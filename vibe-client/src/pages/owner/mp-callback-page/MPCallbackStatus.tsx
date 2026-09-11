import { Link } from 'react-router-dom';
import { CheckCircle2, XCircle } from 'lucide-react';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function MPCallbackStatus({
  status,
  returnPath,
}: {
  status: 'processing' | 'success' | 'error';
  returnPath: string;
}) {
  return (
    <div className="space-y-4 text-center">
      {status === 'processing' && (
        <>
          <LoadingSpinner size="lg" />
          <p className="text-sm text-text-secondary">{t.mp.connecting}</p>
        </>
      )}
      {status === 'success' && (
        <>
          <CheckCircle2 className="mx-auto size-10 text-green-400" />
          <p className="text-sm font-medium text-text-primary">{t.mp.connectSuccess}</p>
        </>
      )}
      {status === 'error' && (
        <>
          <XCircle className="mx-auto size-10 text-error-text" />
          <p className="text-sm font-medium text-text-primary">{t.mp.connectError}</p>
          <Link
            to={returnPath}
            replace
            className="text-sm font-medium text-primary-400 underline-offset-4 transition-colors hover:text-primary-300 hover:underline"
          >
            {t.common.back}
          </Link>
        </>
      )}
    </div>
  );
}
