import { useEffect, useRef, useState } from 'react';
import { loadGoogleIdentityServices } from '@/shared/lib/googleIdentity';
import { env } from '@/shared/lib/env';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useGoogleSignIn } from '../hooks/useGoogleSignIn';
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
 */
export function GoogleSignInButton() {
  const clientId = env.VITE_GOOGLE_CLIENT_ID;
  const frameRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);
  const { mutate } = useGoogleSignIn();
  const scale = useOverlayScale(frameRef, GIS_BUTTON_WIDTH, GIS_BUTTON_HEIGHT);

  useEffect(() => {
    if (!clientId) return;

    let cancelled = false;

    loadGoogleIdentityServices()
      .then((google) => {
        if (cancelled || !containerRef.current) return;

        google.accounts.id.initialize({
          client_id: clientId,
          callback: (response) => {
            mutate(response.credential);
          },
          ux_mode: 'popup',
          auto_select: false,
          use_fedcm_for_prompt: true,
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
  }, [clientId, mutate]);

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
