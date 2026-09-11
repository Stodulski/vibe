import type { StateCreator } from 'zustand';
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
  setUser: (user) => {
    set({ user });
  },
  setCsrfToken: (token) => {
    set({ csrfToken: token });
  },
  logout: () => {
    safeLocalStorage.remove(STORAGE_KEYS.SELECTED_COMPLEX_ID);
    safeSessionStorage.remove(STORAGE_KEYS.MP_CODE_VERIFIER);
    safeSessionStorage.remove(STORAGE_KEYS.MP_RETURN_PATH);
    set({ user: null, csrfToken: null });
  },
});
