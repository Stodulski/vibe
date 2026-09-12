import type { StateCreator } from 'zustand';
import * as Sentry from '@sentry/react';
import type { User } from '@/shared/types/api.types';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { safeLocalStorage, safeSessionStorage } from '@/shared/lib/safeStorage';

export interface AuthSlice {
  user: User | null;
  csrfToken: string | null;
  setUser: (user: User | null) => void;
  setCsrfToken: (token: string | null) => void;
  logout: () => void;
}

export const createAuthSlice: StateCreator<AuthSlice> = (set) => ({
  user: null,
  csrfToken: null,
  // The one place session identity changes — `useAuthSuccessHandler`
  // (login), `useAuth` (session bootstrap on reload) and `useUpdateProfile`
  // all call this instead of touching Sentry themselves, so every event
  // reported while a session is active carries the same `id`. No email or
  // name: Sentry only ever gets the id, never PII.
  setUser: (user) => {
    Sentry.setUser(user ? { id: user.id } : null);
    set({ user });
  },
  setCsrfToken: (token) => {
    set({ csrfToken: token });
  },
  logout: () => {
    safeLocalStorage.remove(STORAGE_KEYS.SELECTED_COMPLEX_ID);
    safeSessionStorage.remove(STORAGE_KEYS.MP_CODE_VERIFIER);
    safeSessionStorage.remove(STORAGE_KEYS.MP_RETURN_PATH);
    Sentry.setUser(null);
    set({ user: null, csrfToken: null });
  },
});
