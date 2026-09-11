import type { StoreApi } from 'zustand';
import { createAuthSlice, type AuthSlice } from './auth.slice';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import type { User } from '@/shared/types/api.types';

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

const mockUser: User = {
  id: 'user-1',
  email: 'test@example.com',
  first_name: 'Juan',
  last_name: 'Perez',
  role: 'owner',
  phone: '1123456789',
  is_active: true,
  email_verified: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

describe('authSlice', () => {
  it('has null user initially', () => {
    const store = createStore();
    expect(store.getState().user).toBeNull();
  });

  it('has null csrfToken initially', () => {
    const store = createStore();
    expect(store.getState().csrfToken).toBeNull();
  });

  it('setUser stores the given user', () => {
    const store = createStore();
    store.setUser(mockUser);
    expect(store.getState().user).toEqual(mockUser);
  });

  it('setUser can clear the user back to null', () => {
    const store = createStore();
    store.setUser(mockUser);
    store.setUser(null);
    expect(store.getState().user).toBeNull();
  });

  it('setCsrfToken stores the given token', () => {
    const store = createStore();
    store.setCsrfToken('test-csrf-token');
    expect(store.getState().csrfToken).toBe('test-csrf-token');
  });

  it('logout clears user and csrfToken', () => {
    const store = createStore();
    store.setUser(mockUser);
    store.setCsrfToken('test-csrf-token');

    store.logout();

    expect(store.getState().user).toBeNull();
    expect(store.getState().csrfToken).toBeNull();
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
});
