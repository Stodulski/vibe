import type { StateCreator } from 'zustand';
import * as Sentry from '@sentry/react';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { safeLocalStorage, safeSessionStorage } from '@/shared/lib/safeStorage';
import { purgeApiCache } from '@/shared/lib/apiCache';

/**
 * What a session is, minus the part that is server state.
 *
 * The signed-in `User` used to live here too. It is answered by
 * `GET /auth/me`, so it belongs in the React Query cache like every other
 * server answer — see `features/auth/hooks/session.ts`. What is left is the
 * in-memory CSRF token, which no endpoint can be asked for on its own.
 */
export interface AuthSlice {
  csrfToken: string | null;
  setCsrfToken: (token: string | null) => void;
  logout: () => void;
}

export const createAuthSlice: StateCreator<AuthSlice> = (set) => ({
  csrfToken: null,
  setCsrfToken: (token) => {
    set({ csrfToken: token });
  },
  logout: () => {
    safeLocalStorage.remove(STORAGE_KEYS.SELECTED_COMPLEX_ID);
    safeSessionStorage.remove(STORAGE_KEYS.MP_CODE_VERIFIER);
    safeSessionStorage.remove(STORAGE_KEYS.MP_RETURN_PATH);
    Sentry.setUser(null);
    // The service worker's API cache outlives the in-memory store, so
    // clearing state is not enough to end a session on a shared device.
    purgeApiCache();
    set({ csrfToken: null });
  },
});
