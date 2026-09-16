import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { renderHook, act } from '@testing-library/react';
import { useCrossTabLogout } from './useCrossTabLogout';
import { queryKeys } from '@/shared/lib/queryKeys';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';

const mockLogoutLocal = vi.fn();
const mockNavigate = vi.fn();

// Selector-aware, like the real zustand store: every call site reads one
// atomic slice now (STORE-02), not the whole state.
vi.mock('@/shared/stores', () => ({
  useStore: (selector: (s: { logoutLocal: typeof mockLogoutLocal }) => unknown) =>
    selector({ logoutLocal: mockLogoutLocal }),
}));

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return { ...actual, useNavigate: () => mockNavigate };
});

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function wrapperFor(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  };
}

function dispatchLogoutBroadcast() {
  window.dispatchEvent(
    new StorageEvent('storage', { key: STORAGE_KEYS.SESSION_LOGOUT_BROADCAST, newValue: String(Date.now()) }),
  );
}

function renderCrossTabLogout(queryClient: QueryClient) {
  return renderHook(
    () => {
      useCrossTabLogout();
    },
    { wrapper: wrapperFor(queryClient) },
  );
}

beforeEach(() => {
  mockLogoutLocal.mockClear();
  mockNavigate.mockClear();
});

afterEach(() => {
  window.history.pushState({}, '', '/');
});

describe('useCrossTabLogout', () => {
  it('reacts to the broadcast key when this tab is authenticated: tears down locally, clears the cache, and redirects', () => {
    const queryClient = makeClient();
    queryClient.setQueryData(queryKeys.auth.me, { user: { id: 'u1' }, pendingEmail: null });
    const clearSpy = vi.spyOn(queryClient, 'clear');

    renderCrossTabLogout(queryClient);

    act(() => {
      dispatchLogoutBroadcast();
    });

    // The store's own broadcasting `logout()` must not be called here — that
    // would write the key again and bounce back to the tab that started it.
    expect(mockLogoutLocal).toHaveBeenCalledTimes(1);
    expect(clearSpy).toHaveBeenCalled();
    // `loginUrlPreserving` appends `?from=` for the current path so a login
    // afterwards returns here — this test's tab sits on "/".
    expect(mockNavigate).toHaveBeenCalledWith('/login?from=%2F', { replace: true });
  });

  it('reacts when this tab is anonymous but sitting on a protected route', () => {
    const queryClient = makeClient();
    window.history.pushState({}, '', '/dashboard');

    renderCrossTabLogout(queryClient);

    act(() => {
      dispatchLogoutBroadcast();
    });

    expect(mockLogoutLocal).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith('/login?from=%2Fdashboard', { replace: true });
  });

  it('does nothing for an anonymous visitor on a public booking page', () => {
    const queryClient = makeClient();
    window.history.pushState({}, '', '/club-padel-norte');

    renderCrossTabLogout(queryClient);

    act(() => {
      dispatchLogoutBroadcast();
    });

    expect(mockLogoutLocal).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('ignores a storage event for an unrelated key', () => {
    const queryClient = makeClient();
    queryClient.setQueryData(queryKeys.auth.me, { user: { id: 'u1' }, pendingEmail: null });

    renderCrossTabLogout(queryClient);

    act(() => {
      window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated-key', newValue: 'x' }));
    });

    expect(mockLogoutLocal).not.toHaveBeenCalled();
  });
});
