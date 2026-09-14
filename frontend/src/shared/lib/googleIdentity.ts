/**
 * Minimal typed surface of Google Identity Services' `google.accounts.id` —
 * only what `GoogleSignInButton` needs (`initialize` + `renderButton`). No
 * official types package is added for ~15 lines of API surface; see
 * https://developers.google.com/identity/gsi/web/reference/js-reference.
 */
interface GoogleCredentialResponse {
  credential: string;
}

export interface GoogleIdConfiguration {
  client_id: string;
  /**
   * Only ever called in `ux_mode: 'popup'`. In `'redirect'` mode Google POSTs
   * the credential to {@link GoogleIdConfiguration.login_uri} instead and this
   * page is navigated away from, so the button passes no callback at all.
   */
  callback?: (response: GoogleCredentialResponse) => void;
  ux_mode?: 'popup' | 'redirect';
  /**
   * Where `ux_mode: 'redirect'` POSTs `credential` + `g_csrf_token` as
   * `application/x-www-form-urlencoded`. Required in redirect mode, ignored in
   * popup mode, and must be registered as an authorized redirect URI in the
   * Google Cloud Console for this client id.
   */
  login_uri?: string;
  auto_select?: boolean;
  use_fedcm_for_prompt?: boolean;
}

export interface GoogleButtonConfiguration {
  theme?: 'outline' | 'filled_blue' | 'filled_black';
  size?: 'large' | 'medium' | 'small';
  text?: 'signin_with' | 'signup_with' | 'continue_with' | 'signin';
  locale?: string;
  width?: number;
}

interface GoogleAccountsId {
  initialize: (config: GoogleIdConfiguration) => void;
  renderButton: (parent: HTMLElement, options: GoogleButtonConfiguration) => void;
}

export interface GoogleNamespace {
  accounts: { id: GoogleAccountsId };
}

declare global {
  interface Window {
    google?: GoogleNamespace;
  }
}

const SCRIPT_SRC = 'https://accounts.google.com/gsi/client';

let loadPromise: Promise<GoogleNamespace> | undefined;

/**
 * Injects the Google Identity Services script once and resolves with
 * `window.google` once `google.accounts.id` exists.
 *
 * A second call while the first is still in flight (or already resolved)
 * reuses the same promise instead of injecting a second `<script>` tag. A
 * failed load removes the stale tag and clears the cached promise, so a
 * later call — e.g. a fresh mount of `GoogleSignInButton` after the user
 * reloads — appends a genuinely new `<script>` and gets a real retry
 * instead of a permanently rejected promise.
 */
export function loadGoogleIdentityServices(): Promise<GoogleNamespace> {
  if (window.google?.accounts.id) return Promise.resolve(window.google);

  if (!loadPromise) {
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${SCRIPT_SRC}"]`);
    const script = existing ?? document.createElement('script');

    loadPromise = new Promise<GoogleNamespace>((resolve, reject) => {
      const handleLoad = () => {
        if (window.google?.accounts.id) {
          resolve(window.google);
        } else {
          reject(new Error('Google Identity Services script loaded without window.google.accounts.id'));
        }
      };
      const handleError = () => {
        reject(new Error('Failed to load the Google Identity Services script'));
      };

      script.addEventListener('load', handleLoad, { once: true });
      script.addEventListener('error', handleError, { once: true });

      if (!existing) {
        script.src = SCRIPT_SRC;
        script.async = true;
        script.defer = true;
        document.head.appendChild(script);
      }
    }).catch((error: unknown) => {
      loadPromise = undefined;
      script.remove();
      throw error;
    });
  }

  return loadPromise;
}
