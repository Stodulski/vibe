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
          <p className="text-text-primary text-sm font-medium">{t.mp.connectError}</p>
          <Link
            to={returnPath}
            replace
            className="text-primary-400 hover:text-primary-300 text-sm font-medium underline-offset-4 transition-colors hover:underline"
          >
            {t.common.back}
          </Link>
        </>
      )}
    </div>
  );
}
