import { useState } from 'react';
import { Turnstile } from '@marsidev/react-turnstile';
import { useBookingConfig } from '../../config';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

interface TurnstileFieldProps {
  /** Called with the solved token, or `undefined` once it is gone (expired, errored). */
  onTokenChange: (token: string | undefined) => void;
}

/**
 * Cloudflare Turnstile challenge of the complex page, whose token takes the
 * slot hold when a court is picked (`POST /book/holds`). The booking package's
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
 *
 * A token is single use. The page that owns the widget gets a fresh one by
 * remounting it under a new `key` (see `useHoldChallenge`).
 */
export function TurnstileField({ onTokenChange }: TurnstileFieldProps) {
  const { turnstileSiteKey } = useBookingConfig();
  const [challengeFailed, setChallengeFailed] = useState(false);
  const [solved, setSolved] = useState(false);

  if (!turnstileSiteKey) return null;

  return (
    <div className="space-y-1.5">
      <div hidden={solved} data-testid="turnstile-slot">
        <Turnstile
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
