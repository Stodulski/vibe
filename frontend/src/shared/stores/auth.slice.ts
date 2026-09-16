import type { StateCreator } from 'zustand';
import { setUser } from '@/shared/lib/observability';
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
  /**
   * The same teardown as {@link AuthSlice.logout}, without the cross-tab
   * broadcast.
   *
   * `logout` writes `STORAGE_KEYS.SESSION_LOGOUT_BROADCAST` so every *other*
   * open tab ends its own session too (see `useCrossTabLogout`). The tab
   * that is *reacting* to that broadcast must tear down through this instead
   * — calling `logout` there would write the key again, and since the
   * `storage` event never fires in the tab that wrote it, that write would
   * fire in every *other* tab (including the one that started it), which
   * would call `logout` again, and so on forever.
   */
  logoutLocal: () => void;
}

// A plain `Date.now()` string can repeat across two `logout()` calls that
// land in the same millisecond (seen in tests, and possible in production on
// a fast machine) — the `storage` event only fires on a *change*, so a
// repeated value would silently fail to notify other tabs. The counter
// guarantees a new value every call regardless of timer resolution.
let broadcastSequence = 0;

export const createAuthSlice: StateCreator<AuthSlice> = (set) => {
  const teardown = () => {
    safeLocalStorage.remove(STORAGE_KEYS.SELECTED_COMPLEX_ID);
    safeSessionStorage.remove(STORAGE_KEYS.MP_CODE_VERIFIER);
    safeSessionStorage.remove(STORAGE_KEYS.MP_RETURN_PATH);
    setUser(null);
    // The service worker's API cache outlives the in-memory store, so
    // clearing state is not enough to end a session on a shared device.
    purgeApiCache();
    set({ csrfToken: null });
  };

  return {
    csrfToken: null,
    setCsrfToken: (token) => {
      set({ csrfToken: token });
    },
    logout: () => {
      broadcastSequence += 1;
      safeLocalStorage.set(STORAGE_KEYS.SESSION_LOGOUT_BROADCAST, `${String(Date.now())}-${String(broadcastSequence)}`);
      teardown();
    },
    logoutLocal: () => {
      teardown();
    },
  };
};
