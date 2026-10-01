import { generateCodeVerifier, generateCodeChallenge } from './pkce';
import { env } from './env';

const ENV_MP_APP_ID = env.VITE_MP_APP_ID ?? '';

/**
 * One prepared MercadoPago authorization attempt: the PKCE pair plus the
 * random `state` nonce that identifies it on the way back. Nothing here is
 * persisted — see `saveMPOAuthSession`, called on click.
 */
export interface MPAuthAttempt {
  state: string;
  verifier: string;
  challenge: string;
}

/**
 * Build the MercadoPago OAuth authorization URL synchronously from a
 * prepared attempt.
 *
 * @param appId The app id from `GET .../mp/status` (`app_id`, the one the API
 * can actually exchange a code with). Falls back to the build-time
 * `VITE_MP_APP_ID` only when the response has none — never the other way
 * around, since a client built against a stale app id would send the seller
 * through an authorization the API cannot complete.
 */
export function buildMPAuthUrl(attempt: MPAuthAttempt, appId?: string): string {
  const redirectUri = `${window.location.origin}/settings/mp/callback`;
  const params = new URLSearchParams({
    client_id: appId ?? ENV_MP_APP_ID,
    response_type: 'code',
    platform_id: 'mp',
    redirect_uri: redirectUri,
    // A random nonce, so the complexId never travels through MercadoPago.
    state: attempt.state,
    code_challenge: attempt.challenge,
    code_challenge_method: 'S256',
  });

  return `https://auth.mercadopago.com/authorization?${params}`;
}

/**
 * Prepare a new attempt (async: the S256 challenge is a digest). Call when
 * the status loads and use the result synchronously on click.
 */
export async function createMPAuthAttempt(): Promise<MPAuthAttempt> {
  const verifier = generateCodeVerifier();
  const challenge = await generateCodeChallenge(verifier);
  return { state: crypto.randomUUID(), verifier, challenge };
}
