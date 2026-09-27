import { useSearchParams } from 'react-router-dom';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { LoadingSpinner } from '@/shared/components/common/LoadingSpinner';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useGoogleFinish } from '../hooks/useGoogleFinish';

const t = ES_AR;

/**
 * Where Google's authorization server lands the browser after the account
 * chooser: `/auth/google/callback?code=…&state=…`, or `?error=…` when the
 * visitor backed out or Google refused outright. `GET
 * {VITE_API_URL}/auth/google/start` (see `GoogleSignInButton`) is what sends
 * the browser there in the first place.
 *
 * {@link useGoogleFinish} spends the code and state immediately. Every
 * outcome — success, `needs_profile`, or a failure — navigates away with a
 * `replace`, which is also what takes `?code=` and `?state=` out of the
 * address bar, so Back never lands here to resubmit them. Nothing on this
 * page is interactive and nothing stays, so the only thing to render is the
 * wait.
 */
export default function GoogleCallbackPage() {
  usePageTitle(t.auth.googleReturnTitle);
  const [searchParams] = useSearchParams();

  useGoogleFinish(searchParams.get('code'), searchParams.get('state'), searchParams.get('error'));

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
