/**
 * Centralized storage keys — prevents key collisions and makes
 * it easy to find all localStorage/sessionStorage usage.
 */
export const STORAGE_KEYS = {
  SELECTED_COMPLEX_ID: 'selectedComplexId',
  BOOKING_CLIENT_DATA: 'vibe_booking_client',
  THEME: 'vibe-theme',
  MP_CODE_VERIFIER: 'mp_code_verifier',
  MP_RETURN_PATH: 'mp_return_path',
} as const;
