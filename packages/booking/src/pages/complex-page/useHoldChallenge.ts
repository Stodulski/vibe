import { useCallback, useState } from 'react';
import { useBookingConfig } from '../../config';

/** The Turnstile challenge of the complex page, as taking a hold sees it. */
export interface HoldChallenge {
  /** True when the storefront has a site key: no hold without a token. */
  required: boolean;
  token: string | undefined;
  /** Throws the spent token away and asks for a fresh one. */
  reset: () => void;
}

/**
 * The state behind the invisible Turnstile widget of the complex page: its
 * token, and a generation the page uses as the widget's `key`. A token is
 * single use, so `reset` drops it and bumps the generation, which mounts a
 * fresh widget that solves a new challenge.
 */
export function useHoldChallenge() {
  const { turnstileSiteKey } = useBookingConfig();
  const [token, setToken] = useState<string>();
  const [generation, setGeneration] = useState(0);
  const reset = useCallback(() => {
    setToken(undefined);
    setGeneration((current) => current + 1);
  }, []);
  const challenge: HoldChallenge = { required: Boolean(turnstileSiteKey), token, reset };
  return { challenge, generation, onTokenChange: setToken };
}
