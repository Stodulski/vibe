import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { createElement } from 'react';
import { toast } from 'sonner';
import { createWrapper } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('../api/auth.api', () => ({
  authApi: {
    login: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), dismiss: vi.fn() } }));

vi.mock('@/shared/stores', () => ({
  useStore: () => ({
    setUser: vi.fn(),
    setCsrfToken: vi.fn(),
  }),
}));

/** Mutates once with a rejected `authApi.login` and waits for the error state. */
async function triggerLoginError(backendError: unknown, options?: { resetTurnstile?: () => void }) {
  const { authApi } = await import('../api/auth.api');
  vi.mocked(authApi.login).mockRejectedValueOnce(backendError);

  const { useLogin } = await import('./useLogin');
  const { result } = renderHook(() => useLogin(options), { wrapper: createWrapper(['/login']) });

  result.current.mutate({ email: 'juan@test.com', password: 'wrong-password' });

  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

describe('useLogin — onError', () => {
  it('shows the rate-limit message on a 429', async () => {
    await triggerLoginError(await makeConsumedHttpError(429, {}));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.rateLimitError);
  });

  it('shows the generic invalid-credentials message (with the unverified-email hint) on any other HTTP status', async () => {
    await triggerLoginError(await makeConsumedHttpError(401, {}));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.invalidCredentials, {
      description: ES_AR.auth.checkInboxIfUnverified,
    });
  });

  // onError used to read error.response.status directly, which crashes for
  // anything that isn't ky's HTTPError (a dropped connection throws a plain
  // TypeError with no .response) — the login form would silently break
  // instead of showing any error to the user.
  it('shows the generic invalid-credentials message instead of crashing on a network error', async () => {
    await triggerLoginError(new TypeError('Failed to fetch'));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.invalidCredentials, {
      description: ES_AR.auth.checkInboxIfUnverified,
    });
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.login).mockReset();
  });
});

describe('useLogin — onSuccess', () => {
  async function triggerLoginSuccess() {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.login).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    const { useLogin } = await import('./useLogin');
    // No `state` on this entry — location.state is null, exactly like a
    // direct visit to /login (not one redirected here by ProtectedRoute).
    const { result } = renderHook(() => useLogin(), { wrapper: createWrapper(['/login']) });

    result.current.mutate({ email: 'juan@test.com', password: 'correct-password' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    return result;
  }

  // onSuccess used to read location.state.from without guarding location.state
  // itself, which is null (not undefined) on a direct visit to /login. That
  // threw inside onSuccess, which TanStack Query then treated as the whole
  // mutation failing — so a login that had actually just succeeded still
  // fired onError's "invalid credentials" toast.
  it('does not show the invalid-credentials toast on a successful login', async () => {
    await triggerLoginSuccess();

    expect(toast.error).not.toHaveBeenCalled();
  });

  // <Toaster> lives above the router (in Providers), so it survives the
  // post-login navigation — an error toast from an earlier failed attempt
  // in the same session would otherwise still be on screen afterwards.
  it('dismisses any lingering toast from a previous failed attempt', async () => {
    await triggerLoginSuccess();

    expect(toast.dismiss).toHaveBeenCalled();
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.login).mockReset();
  });
});

describe('useLogin — location.state.from validation (M9)', () => {
  /** A wrapper carrying real `location.state`, not just a path string. */
  function createWrapperWithState(state: unknown) {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    return function Wrapper({ children }: { children: React.ReactNode }) {
      return createElement(
        QueryClientProvider,
        { client: queryClient },
        createElement(MemoryRouter, { initialEntries: [{ pathname: '/login', state }] }, children),
      );
    };
  }

  async function triggerLoginSuccessWithState(state: unknown) {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.login).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    const { useLogin } = await import('./useLogin');
    const { result } = renderHook(() => useLogin(), { wrapper: createWrapperWithState(state) });

    result.current.mutate({ email: 'juan@test.com', password: 'correct-password' });

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });
    return result;
  }

  // An `as` cast used to trust `location.state.from.pathname` outright.
  // `isSafeRedirect` calls `.startsWith` on it with no type guard, so a
  // `from.pathname` that wasn't a string (a malformed history entry, or a
  // future caller passing the wrong shape) threw inside onSuccess — which
  // TanStack Query treated as the whole mutation failing, firing onError's
  // "invalid credentials" toast on top of a login that had just succeeded.
  it('does not crash and falls back to the default route when location.state.from.pathname is not a string', async () => {
    await triggerLoginSuccessWithState({ from: { pathname: 123 } });

    expect(toast.error).not.toHaveBeenCalled();
  });

  it('still redirects to a safe `from` path when location.state is well-formed', async () => {
    const result = await triggerLoginSuccessWithState({ from: { pathname: '/bookings' } });

    expect(toast.error).not.toHaveBeenCalled();
    expect(result.current.isSuccess).toBe(true);
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.login).mockReset();
  });
});

describe('useLogin — Turnstile 422 handling', () => {
  it('shows the turnstile-specific message instead of the generic invalid-credentials one', async () => {
    await triggerLoginError(await makeConsumedHttpError(422, { error: { turnstile_token: 'required' } }));

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileRequired);
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.auth.invalidCredentials, expect.anything());
  });

  it('translates every documented turnstile_token code', async () => {
    await triggerLoginError(await makeConsumedHttpError(422, { error: { turnstile_token: 'invalid' } }));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileInvalid);

    await triggerLoginError(await makeConsumedHttpError(422, { error: { turnstile_token: 'unavailable' } }));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileUnavailable);
  });

  it('resets the widget on a turnstile failure — tokens are single-use', async () => {
    const resetTurnstile = vi.fn();

    await triggerLoginError(await makeConsumedHttpError(422, { error: { turnstile_token: 'invalid' } }), {
      resetTurnstile,
    });

    expect(resetTurnstile).toHaveBeenCalledTimes(1);
  });

  it('also resets the widget on an unrelated failure — a solved token is burned by any failed submit', async () => {
    const resetTurnstile = vi.fn();

    await triggerLoginError(await makeConsumedHttpError(401, {}), { resetTurnstile });

    expect(resetTurnstile).toHaveBeenCalledTimes(1);
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.login).mockReset();
  });
});
