import { env } from '@/shared/lib/env';
import { ES_AR } from '@/shared/i18n/es_AR';
import { GoogleSignInButton } from './GoogleSignInButton';
import { InAppBrowserNotice } from './InAppBrowserNotice';
import { isInAppBrowser } from '../lib/inAppBrowser';

const t = ES_AR;

/**
 * Divider + "Continuar con Google" button shared by `LoginForm` and step 1
 * of `RegisterForm`. Renders nothing when `VITE_GOOGLE_CLIENT_ID` is unset,
 * so a deployment with no Google client id sees no divider either.
 *
 * Inside an in-app browser (Instagram, Facebook, Messenger, LINE, TikTok, or
 * a generic Android WebView), Google refuses to let its OAuth endpoint run
 * at all — see `inAppBrowser.ts`. The Google button is replaced with a
 * notice pointing the visitor at a real browser; email/password sign-in
 * keeps working exactly as it does everywhere else.
 */
export function GoogleSignInSection() {
  if (!env.VITE_GOOGLE_CLIENT_ID) return null;

  return (
    <div className="!mt-4 space-y-4">
      <div className="flex items-center gap-3" role="separator" aria-label={t.auth.orDivider}>
        <div className="bg-border-subtle h-px flex-1" />
        <span className="text-text-tertiary text-xs" aria-hidden="true">
          {t.auth.orDivider}
        </span>
        <div className="bg-border-subtle h-px flex-1" />
      </div>
      {isInAppBrowser(navigator.userAgent) ? <InAppBrowserNotice /> : <GoogleSignInButton />}
    </div>
  );
}
