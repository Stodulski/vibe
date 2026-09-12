import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { makeConsumedHttpError, makeUser } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createQueryWrapper } from '@/test/test-utils';

const bootUser = makeUser({ id: 'u1', first_name: 'Juan' });

const { mockSetCsrfToken } = vi.hoisted(() => ({
  mockSetCsrfToken: vi.fn(),
}));

vi.mock('../api/auth.api', () => ({
  authApi: {
    login: vi.fn().mockResolvedValue({ user: { id: 'u1', role: 'owner' }, csrf_token: 'tok' }),
    register: vi.fn().mockResolvedValue({ message: 'ok' }),
    logout: vi.fn().mockResolvedValue({}),
    getMe: vi.fn().mockResolvedValue({ user: { id: 'u1', first_name: 'Juan' }, csrf_token: 'tok' }),
  },
}));

vi.mock('@/shared/stores', () => ({
  useStore: Object.assign(
    (selector: (s: { csrfToken: string; setCsrfToken: typeof mockSetCsrfToken; logout: () => void }) => unknown) =>
      selector({ csrfToken: 'test', setCsrfToken: mockSetCsrfToken, logout: vi.fn() }),
    { getState: () => ({ csrfToken: 'test', setCsrfToken: mockSetCsrfToken }) },
  ),
}));

vi.mock('@/shared/lib/queryKeys', () => ({
  queryKeys: { auth: { me: ['auth', 'me'] } },
}));

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
  useLocation: () => ({ state: null, pathname: '/login' }),
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

describe('useAuth', () => {
  it('returns isAuthenticated false when no user', async () => {
    const { useAuth } = await import('./useAuth');
    const { result } = renderHook(() => useAuth(), { wrapper: createQueryWrapper() });
    expect(result.current.isAuthenticated).toBe(false);
  });

  it('returns user as null initially', async () => {
    const { useAuth } = await import('./useAuth');
    const { result } = renderHook(() => useAuth(), { wrapper: createQueryWrapper() });
    expect(result.current.user).toBeNull();
  });

  // DATA-11: the session's user *is* this query's data. The store used to
  // hold a copy and the query deliberately returned `null`, so a cache entry
  // could never speak for the session; now it is the only thing that does.
  it('reports the user the session cache already holds, before any fetch resolves', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(['auth', 'me'], bootUser);

    const { useAuth } = await import('./useAuth');
    const { result } = renderHook(() => useAuth(), {
      wrapper: ({ children }) => createElement(QueryClientProvider, { client: queryClient }, children),
    });

    expect(result.current.user).toEqual(bootUser);
    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.isLoading).toBe(false);
  });

  // A page load boots from GET /auth/me, which carries the CSRF token for the
  // access token the cookie holds. Nothing here may spend the refresh token:
  // that is `bootstrapSession`'s job, and only once /auth/me answered 401.
  // The real `ky` client runs here (no `@/shared/lib/ky` mock): a spy on
  // `refreshAccessToken` proves it was never invoked, while the MSW handler
  // for `GET auth/me` below answers the real network call `bootstrapSession`
  // makes.
  it('answers the bootstrapped session without refreshing the tokens', async () => {
    const ky = await import('@/shared/lib/ky');
    const refreshAccessToken = vi.spyOn(ky, 'refreshAccessToken');
    mockSetCsrfToken.mockClear();
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: bootUser, csrf_token: 'from-me' })));

    const { useAuth } = await import('./useAuth');
    const { result } = renderHook(() => useAuth(), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.user).toEqual(bootUser);
    });
    expect(mockSetCsrfToken).toHaveBeenCalledWith('from-me');
    expect(refreshAccessToken).not.toHaveBeenCalled();
  });

  // The reason DATA-11 exists. With `user` in the store the query was
  // `enabled: !user`, so once a session was read it was never read again for
  // as long as the tab stayed open — a deactivated account, a changed role or
  // a renamed profile kept showing the boot-time answer. Now the session is an
  // ordinary cache entry, and invalidating it re-reads it.
  it('re-reads the session when its cache entry is invalidated', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: bootUser, csrf_token: 'from-me' })));

    const { useAuth } = await import('./useAuth');
    const { result } = renderHook(() => useAuth(), {
      wrapper: ({ children }) => createElement(QueryClientProvider, { client: queryClient }, children),
    });
    await waitFor(() => {
      expect(result.current.user?.first_name).toBe('Juan');
    });

    const renamed = makeUser({ id: 'u1', first_name: 'Juana' });
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: renamed, csrf_token: 'from-me' })));
    await queryClient.invalidateQueries({ queryKey: ['auth', 'me'] });

    await waitFor(() => {
      expect(result.current.user?.first_name).toBe('Juana');
    });
  });
});

describe('useLogin', () => {
  it('returns a mutation with mutate function', async () => {
    const { useLogin } = await import('./useLogin');
    const { result } = renderHook(() => useLogin(), { wrapper: createQueryWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });
});

describe('useLogout', () => {
  it('returns a mutation with mutate function', async () => {
    const { useLogout } = await import('./useLogout');
    const { result } = renderHook(() => useLogout(), { wrapper: createQueryWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });
});

describe('useRegister', () => {
  it('returns a mutation with mutate function', async () => {
    const { useRegister } = await import('./useRegister');
    const { result } = renderHook(() => useRegister(), { wrapper: createQueryWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });

  const registerPayload = {
    first_name: 'Juan',
    last_name: 'Perez',
    email: 'juan@test.com',
    password: 'Sup3rSecret!',
    phone: '1155550000',
  };

  it('onError surfaces the real backend message from error.data instead of the generic fallback', async () => {
    const { authApi } = await import('../api/auth.api');
    const backendError = await makeConsumedHttpError(400, { error: 'El email ya esta registrado' });
    vi.mocked(authApi.register).mockRejectedValueOnce(backendError);

    const { useRegister } = await import('./useRegister');
    const { result } = renderHook(() => useRegister(), { wrapper: createQueryWrapper() });
    result.current.mutate(registerPayload);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith('El email ya esta registrado');
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.auth.registerError);
  });

  it('still shows the rate-limit message on 429, unaffected by the error.data fix', async () => {
    const { authApi } = await import('../api/auth.api');
    const backendError = await makeConsumedHttpError(429, { error: 'ignored for 429' });
    vi.mocked(authApi.register).mockRejectedValueOnce(backendError);

    const { useRegister } = await import('./useRegister');
    const { result } = renderHook(() => useRegister(), { wrapper: createQueryWrapper() });
    result.current.mutate(registerPayload);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.rateLimitError);
  });

  // `isolate: false` (vitest.config.ts) shares the module registry across
  // test files in the same worker; explicitly restoring the resolved
  // default guards against leaking a rejected `register` mock into other
  // files that mock this same resolved module path.
  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.register).mockResolvedValue({ message: 'ok' });
  });
});
