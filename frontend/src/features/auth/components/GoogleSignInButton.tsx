import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { env } from '@/shared/lib/env';
import { ES_AR } from '@/shared/i18n/es_AR';
import { readIntendedFrom } from '../hooks/authSuccess';
import { rememberGoogleReturnPath } from '../lib/googleSignInReturn';
import { GoogleMark } from './GoogleMark';

const t = ES_AR;

/**
 * "Continuar con Google": a plain link into the backend's OIDC
 * authorization-code flow.
 *
 * `GET {VITE_API_URL}/auth/google/start` must be reached by a top-level
 * navigation, never `fetch` or a client-side `onClick`: it sets a host-only
 * `google_oauth_state` cookie and 302s to Google's account chooser, so the
 * click has to leave the page exactly like following any other link would.
 * A plain `<a>` is what makes that true.
 *
 * `VITE_GOOGLE_CLIENT_ID` configures nothing here any more — the backend
 * owns the actual Google client id and secret (see `backend/CLAUDE.md`'s
 * OIDC endpoint checklist) — it is kept only as the switch that decides
 * whether Google sign-in is *offered* on this deployment, the same
 * env-gated pattern as `TurnstileField`. Renders nothing when it is unset.
 */
export function GoogleSignInButton() {
  const location = useLocation();

  // Where this person was heading before they were asked to sign in. Read
  // here, on the page that still knows it, because the click below leaves
  // for Google and comes back on `/auth/google/callback`, a route that has
  // no way to work the destination out for itself — see
  // `rememberGoogleReturnPath`. Kept in step with the location rather than
  // read once on mount: `/login` is a single route instance, so arriving
  // from one protected page and then another does not remount this button.
  useEffect(() => {
    rememberGoogleReturnPath(readIntendedFrom(location.state, location.search));
  }, [location.state, location.search]);

  if (!env.VITE_GOOGLE_CLIENT_ID) return null;

  return (
    <a
      href={`${env.VITE_API_URL}/auth/google/start`}
      className="border-border-interactive bg-background text-foreground hover:border-border-interactive-hover hover:bg-accent focus-visible:ring-ring/50 flex h-11 w-full items-center justify-center gap-2 rounded-full border text-sm font-semibold transition-colors outline-none focus-visible:ring-[3px]"
    >
      <GoogleMark className="size-4" />
      {t.auth.continueWithGoogle}
    </a>
  );
}
