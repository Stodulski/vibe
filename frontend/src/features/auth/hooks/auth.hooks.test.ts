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

const { mockSetUser, mockSetCsrfToken } = vi.hoisted(() => ({
  mockSetUser: vi.fn(),
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
    () => ({
      user: null,
      setUser: mockSetUser,
      setCsrfToken: mockSetCsrfToken,
      csrfToken: 'test',
      logout: vi.fn(),
    }),
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

  // M5: the store is the single source of truth for `user` — Sidebar,
  // RootRedirect and other non-auth consumers read `useStore().user`
  // directly, not this hook's query cache. The old implementation fell back
  // to `query.data?.user` whenever the store had none, so a stale cache
  // entry (e.g. left over after a missed `queryClient.clear()`) could make
  // this hook report someone logged in while every other reader of the
  // store still said logged out.
  it('does not report a user from the query cache when the store itself has none', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(['auth', 'me'], {
      user: { id: 'stale-cached-user', first_name: 'Stale' },
    });

    const { useAuth } = await import('./useAuth');
    const { result } = renderHook(() => useAuth(), {
      wrapper: ({ children }) => createElement(QueryClientProvider, { client: queryClient }, children),
    });

    // Checked synchronously, before the background getMe() fetch this query
    // triggers (enabled: !user) has a chance to resolve.
    expect(result.current.user).toBeNull();
  });

  // A page load boots from GET /auth/me, which carries the CSRF token for the
  // access token the cookie holds. Nothing here may spend the refresh token:
  // that is `bootstrapSession`'s job, and only once /auth/me answered 401.
  // The real `ky` client runs here (no `@/shared/lib/ky` mock): a spy on
  // `refreshAccessToken` proves it was never invoked, while the MSW handler
  // for `GET auth/me` below answers the real network call `bootstrapSession`
  // makes.
  it('feeds the store from the bootstrapped session without refreshing the tokens', async () => {
    const ky = await import('@/shared/lib/ky');
    const refreshAccessToken = vi.spyOn(ky, 'refreshAccessToken');
    mockSetUser.mockClear();
    mockSetCsrfToken.mockClear();
    server.use(http.get('*/auth/me', () => HttpResponse.json({ user: bootUser, csrf_token: 'from-me' })));

    const { useAuth } = await import('./useAuth');
    renderHook(() => useAuth(), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(mockSetUser).toHaveBeenCalledWith(bootUser);
    });
    expect(mockSetCsrfToken).toHaveBeenCalledWith('from-me');
    expect(refreshAccessToken).not.toHaveBeenCalled();
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
