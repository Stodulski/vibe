import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeUser } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';

const mockSentrySetUser = vi.fn();
vi.mock('@sentry/react', () => ({
  setUser: (user: unknown) => {
    mockSentrySetUser(user);
  },
  captureException: vi.fn(),
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), dismiss: vi.fn() } }));

const { identifySession, setSessionUser } = await import('./session');
const { useAuth } = await import('./useAuth');
const { useUpdateProfile } = await import('./useUpdateProfile');
const { useLogout } = await import('./useLogout');

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
}

function wrapperFor(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/settings']}>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  };
}

beforeEach(() => {
  mockSentrySetUser.mockClear();
});

// OBS-05 used to be the store's `setUser`. Moving the user into the query
// cache moved this with it: Sentry still learns who is signed in, and still
// learns nothing else about them.
describe('identifySession', () => {
  it('tells Sentry the signed-in id and nothing else', () => {
    identifySession(makeUser({ id: 'u-7', email: 'juan@test.com', first_name: 'Juan' }));
    expect(mockSentrySetUser).toHaveBeenCalledWith({ id: 'u-7' });
  });

  it('clears the Sentry user for a session that resolved to nobody', () => {
    identifySession(null);
    expect(mockSentrySetUser).toHaveBeenCalledWith(null);
  });
});

describe('the session cache (DATA-11)', () => {
  it('is what useAuth reports once a login seeds it, with no request of its own', () => {
    const queryClient = makeClient();
    setSessionUser(queryClient, makeUser({ id: 'u-1', first_name: 'Ana' }));

    const { result } = renderHook(() => useAuth(), { wrapper: wrapperFor(queryClient) });

    expect(result.current.user?.first_name).toBe('Ana');
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('is written through by a profile update, so the screen shows the new name at once', async () => {
    const queryClient = makeClient();
    setSessionUser(queryClient, makeUser({ id: 'u-1', first_name: 'Ana' }));
    server.use(http.put('*/auth/me', () => HttpResponse.json({ user: makeUser({ id: 'u-1', first_name: 'Anabel' }) })));

    const wrapper = wrapperFor(queryClient);
    const { result } = renderHook(() => ({ auth: useAuth(), update: useUpdateProfile() }), { wrapper });

    result.current.update.mutate({
      first_name: 'Anabel',
      last_name: 'Perez',
      email: 'ana@test.com',
      phone: '1155550000',
    });

    await waitFor(() => {
      expect(result.current.auth.user?.first_name).toBe('Anabel');
    });
  });

  // `queryClient.clear()` alone would leave the `useAuth` still mounted on the
  // way to /login with no data — so it would immediately ask `GET /auth/me`
  // again, and on a shared device the answer could put the person back. The
  // seeded `null` is what settles the session as "signed out" instead.
  it('is left signed out by a logout, and does not re-read itself afterwards', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 5 * 60 * 1000 }, mutations: { retry: false } },
    });
    setSessionUser(queryClient, makeUser({ id: 'u-1', first_name: 'Ana' }));

    const { result } = renderHook(() => ({ auth: useAuth(), logout: useLogout() }), {
      wrapper: wrapperFor(queryClient),
    });
    expect(result.current.auth.user?.first_name).toBe('Ana');

    result.current.logout.mutate();

    await waitFor(() => {
      expect(result.current.auth.isAuthenticated).toBe(false);
    });
    expect(queryClient.getQueryData(queryKeys.auth.me)).toBeNull();
    expect(mockSentrySetUser).toHaveBeenLastCalledWith(null);

    // The default MSW handler for GET /auth/me answers with a user. Nothing
    // may bring it back on its own.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(result.current.auth.user).toBeNull();
  });
});
