import { env } from '@/shared/lib/env';
import { ES_AR } from '@/shared/i18n/es_AR';
import { GoogleSignInButton } from './GoogleSignInButton';

const t = ES_AR;

/**
 * Divider + "Continuar con Google" button shared by `LoginForm` and step 1
 * of `RegisterForm`. Renders nothing when `VITE_GOOGLE_CLIENT_ID` is unset,
 * so a deployment with no Google client id sees no divider either.
 */
export function GoogleSignInSection() {
  if (!env.VITE_GOOGLE_CLIENT_ID) return null;

  return (
    <div className="!mt-4 space-y-4">
      <div className="flex items-center gap-3" role="separator" aria-label={t.auth.orDivider}>
        <div className="h-px flex-1 bg-border-subtle" />
        <span className="text-xs text-text-tertiary" aria-hidden="true">
          {t.auth.orDivider}
        </span>
        <div className="h-px flex-1 bg-border-subtle" />
      </div>
      <GoogleSignInButton />
    </div>
  );
}
