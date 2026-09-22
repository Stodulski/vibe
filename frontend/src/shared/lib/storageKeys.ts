/**
 * Centralized storage keys — prevents key collisions and makes
 * it easy to find all localStorage/sessionStorage usage.
 */
export const STORAGE_KEYS = {
  BOOKING_CLIENT_DATA: 'vibe_booking_client',
  THEME: 'vibe-theme',
  MP_CODE_VERIFIER: 'mp_code_verifier',
  MP_RETURN_PATH: 'mp_return_path',
  // Written by `logout()` (auth.slice.ts) with a changing value on every
  // logout — never read for its value, only its `storage` event, which
  // fires in every *other* tab on this origin (never the writer). That is
  // what tells a second open tab another tab just ended the session — see
  // `useCrossTabLogout`.
  SESSION_LOGOUT_BROADCAST: 'vibe_session_logout_broadcast',
} as const;
