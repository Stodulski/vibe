import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { StoreApi } from 'zustand';
import { createAuthSlice, type AuthSlice } from './auth.slice';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { API_CACHE_NAME } from '@/shared/lib/apiCache';

const mockSetUser = vi.fn<(user: { id: string } | null) => void>();

vi.mock('@sentry/react', () => ({
  setUser: (user: { id: string } | null) => {
    mockSetUser(user);
  },
}));

function createStore() {
  let state: AuthSlice = {} as AuthSlice;
  const setState: StoreApi<AuthSlice>['setState'] = (partial) => {
    if (typeof partial === 'function') {
      state = { ...state, ...partial(state) };
    } else {
      state = { ...state, ...partial };
    }
  };
  const getState: StoreApi<AuthSlice>['getState'] = () => state;
  const store = {
    setState,
    getState,
    getInitialState: getState,
    subscribe: () => () => {
      /* noop unsubscribe */
    },
  } satisfies StoreApi<AuthSlice>;
  const initialState = createAuthSlice(setState, getState, store);
  state = { ...initialState };
  return {
    getState: () => state,
    ...state,
  };
}

describe('authSlice', () => {
  beforeEach(() => {
    mockSetUser.mockClear();
  });

  it('has null csrfToken initially', () => {
    const store = createStore();
    expect(store.getState().csrfToken).toBeNull();
  });

  it('setCsrfToken stores the given token', () => {
    const store = createStore();
    store.setCsrfToken('test-csrf-token');
    expect(store.getState().csrfToken).toBe('test-csrf-token');
  });

  it('logout clears the csrfToken', () => {
    const store = createStore();
    store.setCsrfToken('test-csrf-token');

    store.logout();

    expect(store.getState().csrfToken).toBeNull();
  });

  // DATA-11: the signed-in user is React Query's now (`auth/hooks/session.ts`).
  // The slice must not grow a second copy of it back.
  it('holds no user of its own', () => {
    const store = createStore();
    expect(store.getState()).not.toHaveProperty('user');
    expect(store.getState()).not.toHaveProperty('setUser');
  });

  it('logout removes the persisted complex selection and MercadoPago OAuth state', () => {
    localStorage.setItem(STORAGE_KEYS.SELECTED_COMPLEX_ID, 'complex-123');
    sessionStorage.setItem(STORAGE_KEYS.MP_CODE_VERIFIER, 'verifier');
    sessionStorage.setItem(STORAGE_KEYS.MP_RETURN_PATH, '/dashboard');
    const store = createStore();

    store.logout();

    expect(localStorage.getItem(STORAGE_KEYS.SELECTED_COMPLEX_ID)).toBeNull();
    expect(sessionStorage.getItem(STORAGE_KEYS.MP_CODE_VERIFIER)).toBeNull();
    expect(sessionStorage.getItem(STORAGE_KEYS.MP_RETURN_PATH)).toBeNull();
  });

  it('logout purges the service worker API cache so nothing stays readable on the device', () => {
    const del = vi.fn().mockResolvedValue(true);
    vi.stubGlobal('caches', { delete: del });
    const store = createStore();

    store.logout();

    expect(del).toHaveBeenCalledWith(API_CACHE_NAME);
    vi.unstubAllGlobals();
  });
});

// OBS-05: every place session identity changes must also tell Sentry, so
// reported events carry the id of who was signed in when they happened. The
// signing-in half moved to `identifySession` (see session.test.ts); ending the
// session is still this slice's job.
describe('authSlice Sentry integration', () => {
  beforeEach(() => {
    mockSetUser.mockClear();
  });

  it('logout clears the Sentry user', () => {
    const store = createStore();
    store.logout();
    expect(mockSetUser).toHaveBeenLastCalledWith(null);
  });
});
