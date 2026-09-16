import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { CrossTabLogout } from './CrossTabLogout';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { queryKeys } from '@/shared/lib/queryKeys';

const mockLogoutLocal = vi.fn();

// Selector-aware, like the real zustand store: every call site reads one
// atomic slice now (STORE-02), not the whole state.
vi.mock('@/shared/stores', () => ({
  useStore: (selector: (s: { logoutLocal: typeof mockLogoutLocal }) => unknown) =>
    selector({ logoutLocal: mockLogoutLocal }),
}));

beforeEach(() => {
  mockLogoutLocal.mockClear();
});

describe('CrossTabLogout', () => {
  it('renders nothing, and mounts useCrossTabLogout so the router root reacts to a broadcast logout', () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    // An authenticated session, so useCrossTabLogout's "an anonymous visitor
    // on a public page" early return does not swallow this test's event.
    queryClient.setQueryData(queryKeys.auth.me, { user: { id: 'u1' }, pendingEmail: null });

    const { container } = render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <CrossTabLogout />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(container).toBeEmptyDOMElement();

    // Full behavior (protected-path gating, redirect target, cache clearing)
    // is useCrossTabLogout's own test; this only pins that mounting this
    // component wires the hook up at all.
    act(() => {
      window.dispatchEvent(new StorageEvent('storage', { key: STORAGE_KEYS.SESSION_LOGOUT_BROADCAST, newValue: '1' }));
    });
    expect(mockLogoutLocal).toHaveBeenCalledTimes(1);
  });
});
