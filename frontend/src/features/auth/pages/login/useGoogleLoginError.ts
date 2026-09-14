import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Every `?error=` value `/login` answers with a message, and the sentence it
 * shows for it.
 *
 * The query string is the mechanism because it is the only one the *backend*
 * can use: `POST /api/v1/auth/google/redirect` never renders a page, it 303s
 * to `/login?error=…`, and router state does not survive a redirect from
 * another origin. `useGoogleExchange` writes its own failures the same way so
 * there is one channel rather than two.
 */
const GOOGLE_ERRORS: Record<string, string> = {
  google_rejected: t.auth.googleErrorRejected,
  google_unavailable: t.auth.googleErrorUnavailable,
  google_expired: t.auth.googleErrorExpired,
};

/**
 * The message a Google sign-in failure left in the URL, if any.
 *
 * The parameter is stripped on the first render that sees it — a reload, or
 * a link shared out of the address bar, must not resurrect a failure from
 * minutes ago — but the sentence is copied into state first, so removing it
 * does not blank the banner the person is reading. An `error` value that is
 * not one of ours is cleared just the same and shows nothing.
 */
export function useGoogleLoginError(): string | undefined {
  const [searchParams, setSearchParams] = useSearchParams();
  // Read once, in the initializer, rather than assigned from the effect
  // below: the effect's whole job is to delete the parameter it read, and a
  // `setMessage` there would be a second render chasing the first. Every way
  // into this page carries its `?error=` from the start — a 303 out of the
  // API is a full page load, and `useGoogleExchange` navigates to a route
  // this component is not mounted on — so there is no later value to catch.
  const [message] = useState(() => GOOGLE_ERRORS[searchParams.get('error') ?? '']);

  useEffect(() => {
    if (!searchParams.has('error')) return;

    // Only `error` is dropped: `?from=` carries the page the visitor was
    // trying to reach and `useAuthSuccessHandler` still needs it afterwards.
    setSearchParams(
      (params) => {
        params.delete('error');
        return params;
      },
      { replace: true },
    );
  }, [searchParams, setSearchParams]);

  return message;
}
