import { forwardRef, useImperativeHandle, useRef, useState } from 'react';
import { Turnstile, type TurnstileInstance } from '@marsidev/react-turnstile';
import { env } from '@/shared/lib/env';
import { useTheme } from '@/shared/hooks/useTheme';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export interface TurnstileFieldHandle {
  /**
   * Resets the widget so the next submit attempt gets a fresh token —
   * Turnstile tokens are single-use, so this must run after any failed
   * submit, not only a Turnstile-specific one.
   */
  reset: () => void;
}

interface TurnstileFieldProps {
  /** Called with the solved token, or `undefined` once it's gone (expired, reset, errored, or never solved). */
  onTokenChange: (token: string | undefined) => void;
}

/**
 * Cloudflare Turnstile challenge, shared by the register/login/forgot-password
 * forms.
 *
 * Renders nothing when `VITE_TURNSTILE_SITE_KEY` is unset (`env.ts`), so a
 * self-hosted instance with no Turnstile configured sees no widget, no
 * script injection, and no change in submit behavior.
 *
 * The widget stays out of the way: `appearance: 'interaction-only'` keeps it
 * invisible while Cloudflare's non-interactive check passes on its own, and
 * once a token is in hand the widget is collapsed here as well, so a visitor
 * who did have to click sees the challenge disappear. It comes back only when
 * a fresh token is needed: on expiry, on error, or after `reset()`. The widget
 * is hidden, never unmounted, so its own expiry/refresh cycle keeps running.
 */
export const TurnstileField = forwardRef<TurnstileFieldHandle, TurnstileFieldProps>(function TurnstileField(
  { onTokenChange },
  ref,
) {
  const { theme } = useTheme();
  const widgetRef = useRef<TurnstileInstance>(null);
  const [challengeFailed, setChallengeFailed] = useState(false);
  const [solved, setSolved] = useState(false);

  useImperativeHandle(ref, () => ({
    reset: () => {
      widgetRef.current?.reset();
      setChallengeFailed(false);
      setSolved(false);
      onTokenChange(undefined);
    },
  }));

  const siteKey = env.VITE_TURNSTILE_SITE_KEY;
  if (!siteKey) return null;

  return (
    <div className="space-y-1.5">
      <div hidden={solved} data-testid="turnstile-slot">
        <Turnstile
          ref={widgetRef}
          siteKey={siteKey}
          options={{ theme, size: 'flexible', language: 'es', appearance: 'interaction-only' }}
          onSuccess={(token) => {
            setChallengeFailed(false);
            setSolved(true);
            onTokenChange(token);
          }}
          onExpire={() => {
            setSolved(false);
            onTokenChange(undefined);
          }}
          onError={() => {
            setChallengeFailed(true);
            setSolved(false);
            onTokenChange(undefined);
          }}
        />
      </div>
      {/* Always mounted so the live region exists before the text inside it
          changes — an aria-live element created with content already in it
          is not guaranteed to be announced. */}
      <p aria-live="polite" className="text-xs text-error-text">
        {challengeFailed ? t.auth.turnstileChallengeError : ''}
      </p>
    </div>
  );
});
