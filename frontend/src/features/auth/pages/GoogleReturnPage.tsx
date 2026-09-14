import { useSearchParams } from 'react-router-dom';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useGoogleExchange } from '../hooks/useGoogleExchange';

const t = ES_AR;

/**
 * Where the backend's Google redirect handler lands the browser after it has
 * verified the credential Google POSTed to `/auth/google/callback`.
 *
 * It carries nothing but `?code=` — a single-use, 120 s token — which
 * {@link useGoogleExchange} spends immediately, together with the
 * `g_csrf_token` cookie Google left on this origin, for a real session.
 * Nothing on this page is interactive and nothing stays: every outcome
 * navigates away, so the only thing to render is the wait.
 */
export default function GoogleReturnPage() {
  usePageTitle(t.auth.googleReturnTitle);
  const [searchParams] = useSearchParams();

  useGoogleExchange(searchParams.get('code'));

  return (
    <AuthSplitLayout>
      <div className="auth-card w-full">
        <div className="flex flex-col items-center gap-4 px-0 py-10 text-center">
          <LoadingSpinner size="lg" label={t.auth.googleReturnLoading} />
          <p className="text-text-tertiary text-sm">{t.auth.googleReturnLoading}</p>
        </div>
      </div>
    </AuthSplitLayout>
  );
}
