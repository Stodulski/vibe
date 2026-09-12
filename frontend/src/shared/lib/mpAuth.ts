import { generateCodeVerifier, generateCodeChallenge } from './pkce';
import { safeSessionStorage } from './safeStorage';
import { env } from './env';

const ENV_MP_APP_ID = env.VITE_MP_APP_ID ?? '';

/**
 * Build the MercadoPago OAuth authorization URL synchronously
 * using a pre-computed PKCE challenge.
 *
 * @param appId The app id from `GET .../mp/status` (`app_id`, the one the API
 * can actually exchange a code with). Falls back to the build-time
 * `VITE_MP_APP_ID` only when the response has none — never the other way
 * around, since a client built against a stale app id would send the seller
 * through an authorization the API cannot complete.
 */
export function buildMPAuthUrl(
  complexId: string,
  pkce: { verifier: string; challenge: string },
  appId?: string,
): string {
  // Use a random nonce as state to avoid leaking the complexId.
  const nonce = crypto.randomUUID();
  safeSessionStorage.set('mp_oauth_complex_' + nonce, complexId);

  const redirectUri = `${window.location.origin}/settings/mp/callback`;
  const params = new URLSearchParams({
    client_id: appId ?? ENV_MP_APP_ID,
    response_type: 'code',
    platform_id: 'mp',
    redirect_uri: redirectUri,
    state: nonce,
    code_challenge: pkce.challenge,
    code_challenge_method: 'S256',
  });

  return `https://auth.mercadopago.com/authorization?${params}`;
}

/**
 * Pre-generate PKCE values (async). Call on mount, use result synchronously on click.
 */
export async function generatePKCE(): Promise<{ verifier: string; challenge: string }> {
  const verifier = generateCodeVerifier();
  const challenge = await generateCodeChallenge(verifier);
  return { verifier, challenge };
}
