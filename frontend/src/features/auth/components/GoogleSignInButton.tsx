import { useEffect, useRef, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { loadGoogleIdentityServices } from '@/shared/lib/googleIdentity';
import { env } from '@/shared/lib/env';
import { ES_AR } from '@/shared/i18n/es_AR';
import { readIntendedFrom } from '../hooks/authSuccess';
import { rememberGoogleReturnPath } from '../lib/googleSignInReturn';
import { useOverlayScale } from '../hooks/useOverlayScale';
import { GoogleMark } from './GoogleMark';

const t = ES_AR;

/**
 * The size Google Identity Services draws its `size: 'large'` button at. The
 * width is the largest it accepts (it clamps anything above), and the height
 * is fixed by the size; neither can be made to match the form's own primary
 * button (`h-11 w-full rounded-full`).
 */
export const GIS_BUTTON_WIDTH = 400;
export const GIS_BUTTON_HEIGHT = 40;

/**
 * What the visitor sees: the primary button's shape (`h-11 w-full
 * rounded-full`) in the outline style. Hover and focus styles come from the
 * frame (`group`), because the pointer and the focus both land on the
 * invisible Google layer above, never on this element.
 */
function Decoy() {
  return (
    <span
      aria-hidden="true"
      className="border-border-interactive bg-background text-foreground group-hover:border-border-interactive-hover group-hover:bg-accent group-focus-within:ring-ring/50 flex h-full w-full items-center justify-center gap-2 rounded-full border text-sm font-semibold transition-colors group-focus-within:ring-[3px]"
    >
      <GoogleMark className="size-4" />
      {t.auth.continueWithGoogle}
    </span>
  );
}

/**
 * "Continuar con Google", shaped exactly like the form's primary button.
 *
 * Google Identity Services will only draw its own button, at its own size,
 * and that is the only thing that can start the sign-in flow. So the button
 * the visitor sees is a decoy styled like the primary one, and the real
 * Google button sits over it, fully transparent and scaled to cover it, so
 * every click and every focus lands on Google's element. The decoy is
 * `aria-hidden`: the accessible control is Google's own, which carries its
 * own name and focusability.
 *
 * Renders nothing when `VITE_GOOGLE_CLIENT_ID` is unset — the same env-gated
 * pattern as `TurnstileField`.
 *
 * Deliberately does not enable One Tap: `auto_select: false` and no
 * `prompt()` call anywhere — the rendered button is the only entry point.
 *
 * ## Why `ux_mode: 'redirect'`
 *
 * In popup mode Google opens its own window and hands the credential back to
 * this page through a JS `callback`. On mobile Safari (storage partitioning)
 * and inside in-app webviews that handback never happens: the visitor is left
 * staring at a blank Google page and the sign-in silently dies. Redirect mode
 * has no window to talk back to, so it works everywhere — the trade is that
 * the flow becomes two hops through the app instead of one callback:
 *
 * 1. Google POSTs `credential` + `g_csrf_token` (form-encoded) to `login_uri`
 *    — `/auth/google/callback` on this origin, which Vercel rewrites (and the
 *    Vite proxy forwards in dev/E2E) to `POST /api/v1/auth/google/redirect`.
 *    `login_uri` must be on *this* origin, not the API's: the `g_csrf_token`
 *    Google double-submits is a cookie it sets here.
 * 2. That endpoint always answers with a 303 back to the app:
 *    `/auth/google/return?code=…` on success — where `GoogleReturnPage`
 *    exchanges the single-use code, plus that same cookie read back from
 *    `document.cookie`, for a session — or `/login?error=…` when it refuses.
 *
 * Redirect mode everywhere, not just on mobile: one flow is one thing to keep
 * working, and the desktop popup was never the part that was broken.
 */
export function GoogleSignInButton() {
  const clientId = env.VITE_GOOGLE_CLIENT_ID;
  const frameRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);
  const location = useLocation();
  const scale = useOverlayScale(frameRef, GIS_BUTTON_WIDTH, GIS_BUTTON_HEIGHT);

  // Where this person was heading before they were asked to sign in. It is
  // read here, on the page that still knows it, because the click that starts
  // the flow leaves for Google and comes back somewhere else entirely — see
  // `rememberGoogleReturnPath`. Kept in step with the location rather than
  // read once on mount: `/login` is a single route instance, so arriving from
  // one protected page and then another does not remount this button.
  useEffect(() => {
    rememberGoogleReturnPath(readIntendedFrom(location.state, location.search));
  }, [location.state, location.search]);

  useEffect(() => {
    if (!clientId) return;

    let cancelled = false;

    loadGoogleIdentityServices()
      .then((google) => {
        if (cancelled || !containerRef.current) return;

        google.accounts.id.initialize({
          client_id: clientId,
          ux_mode: 'redirect',
          // Built from the live origin so previews and localhost each post to
          // themselves; production is exactly https://app.vibe.com.ar/auth/google/callback.
          login_uri: `${window.location.origin}/auth/google/callback`,
          auto_select: false,
          // No `callback`: in redirect mode Google navigates away and POSTs
          // the credential to `login_uri`, so nothing here would ever run it.
          // No `use_fedcm_for_prompt` either — it only governs the One Tap
          // `prompt()`, which this component deliberately never calls.
        });

        google.accounts.id.renderButton(containerRef.current, {
          theme: 'outline',
          size: 'large',
          text: 'continue_with',
          locale: 'es',
          width: GIS_BUTTON_WIDTH,
        });
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });

    return () => {
      cancelled = true;
    };
  }, [clientId]);

  if (!clientId) return null;

  if (failed) {
    return <p className="text-text-tertiary text-center text-xs">{t.auth.googleUnavailable}</p>;
  }

  return (
    <div ref={frameRef} className="group relative h-11 w-full">
      <Decoy />
      <div
        ref={containerRef}
        data-testid="google-button-host"
        className="absolute top-0 left-0 origin-top-left opacity-0"
        style={{
          width: GIS_BUTTON_WIDTH,
          height: GIS_BUTTON_HEIGHT,
          transform: `scale(${String(scale.x)}, ${String(scale.y)})`,
        }}
      />
    </div>
  );
}
