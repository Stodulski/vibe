import { useImperativeHandle, useRef, useState, type Ref } from 'react';
import { Turnstile, type TurnstileInstance } from '@marsidev/react-turnstile';
import { useBookingConfig } from '../../config';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

export interface TurnstileFieldHandle {
  /**
   * Resets the widget so the next attempt gets a fresh token. Turnstile
   * tokens are single use: the server spends one on every request it answers.
   */
  reset: () => void;
}

interface TurnstileFieldProps {
  ref?: Ref<TurnstileFieldHandle> | undefined;
  /** Called with the solved token, or `undefined` once it is gone (expired, reset, errored). */
  onTokenChange: (token: string | undefined) => void;
}

/**
 * Cloudflare Turnstile challenge for the confirm step. The booking package's
 * copy of the owner app's `TurnstileField` (`frontend/src/shared/components/
 * common/TurnstileField.tsx`): same library, same options, same behaviour. The
 * package cannot import from the app, so it reads the site key from
 * `BookingConfig` instead of the app's env.
 *
 * Renders nothing without `turnstileSiteKey`, so a storefront without
 * Turnstile loads no script and submits as it always did.
 *
 * `appearance: 'interaction-only'` keeps it invisible while Cloudflare's
 * non-interactive check passes on its own; once a token is in hand it is
 * collapsed here too. It is hidden, never unmounted, so its own expiry and
 * refresh cycle keeps running. The storefront is dark only, hence the theme.
 */
export function TurnstileField({ ref, onTokenChange }: TurnstileFieldProps) {
  const { turnstileSiteKey } = useBookingConfig();
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

  if (!turnstileSiteKey) return null;

  return (
    <div className="space-y-1.5">
      <div hidden={solved} data-testid="turnstile-slot">
        <Turnstile
          ref={widgetRef}
          siteKey={turnstileSiteKey}
          options={{ theme: 'dark', size: 'flexible', language: 'es', appearance: 'interaction-only' }}
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
      {/* Always mounted, so the live region exists before its text changes. */}
      <p aria-live="polite" className="text-error-text text-xs">
        {challengeFailed ? t.publicBooking.turnstileChallengeError : ''}
      </p>
    </div>
  );
}
