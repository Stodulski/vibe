import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { createWrapper } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('../api/auth.api', () => ({
  authApi: {
    googleComplete: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), dismiss: vi.fn() } }));

const mockSetCsrfToken = vi.fn();
// DATA-11: `authSuccess` reads one atomic slice of the store (STORE-02) and
// writes the user into the query cache instead (`./session`).
const mockSetSessionUser = vi.fn();
vi.mock('./session', () => ({
  identifySession: (user: unknown) => user,
  setSessionUser: (_queryClient: unknown, user: unknown) => {
    mockSetSessionUser(user);
  },
}));
vi.mock('@/shared/stores', () => ({
  useStore: (selector: (s: { setCsrfToken: typeof mockSetCsrfToken }) => unknown) =>
    selector({ setCsrfToken: mockSetCsrfToken }),
}));

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

const PAYLOAD = { profile_token: 'a-profile-token', phone: '+541123456789', first_name: 'Juan', last_name: 'Perez' };

async function triggerGoogleCompleteError(backendError: unknown) {
  const { authApi } = await import('../api/auth.api');
  vi.mocked(authApi.googleComplete).mockRejectedValueOnce(backendError);

  const { useGoogleComplete } = await import('./useGoogleComplete');
  const { result } = renderHook(() => useGoogleComplete(), { wrapper: createWrapper(['/register/google']) });

  result.current.mutate(PAYLOAD);

  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

describe('useGoogleComplete — onError', () => {
  it('shows the session-expired message on a 401 (expired profile_token)', async () => {
    await triggerGoogleCompleteError(await makeConsumedHttpError(401, {}));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.googleSessionExpired);
  });

  it('shows the account-exists message on a 409', async () => {
    await triggerGoogleCompleteError(
      await makeConsumedHttpError(409, { title: 'Conflict', detail: 'account already exists' }),
    );
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.googleAccountExists);
  });

  it('shows the rate-limit message on a 429', async () => {
    await triggerGoogleCompleteError(await makeConsumedHttpError(429, {}));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.rateLimitError);
  });

  // 422 field errors are applied to the form by the page's own per-call
  // onError (see GoogleCompleteForm) — the hook itself must stay silent so
  // the person doesn't get both a toast and a field-level message.
  it('does not toast on a 422 — the caller applies field errors instead', async () => {
    await triggerGoogleCompleteError(
      await makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'phone', message: 'phone_invalid' }],
      }),
    );
    expect(toast.error).not.toHaveBeenCalled();
  });

  // A dropped connection gets the generic connectivity message (ERR-04), not
  // this mutation's own googleSignInError fallback — it never reached the
  // server at all.
  it('shows the generic connectivity message for a network error', async () => {
    await triggerGoogleCompleteError(new TypeError('Failed to fetch'));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.common.networkError);
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleComplete).mockReset();
    mockNavigate.mockReset();
  });
});

describe('useGoogleComplete — onSuccess', () => {
  it('behaves exactly like a login success', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleComplete).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    const { useGoogleComplete } = await import('./useGoogleComplete');
    const { result } = renderHook(() => useGoogleComplete(), { wrapper: createWrapper(['/register/google']) });

    result.current.mutate(PAYLOAD);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockSetCsrfToken).toHaveBeenCalledWith('token');
    expect(mockSetSessionUser).toHaveBeenCalledWith(expect.objectContaining({ id: '1' }));
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleComplete).mockReset();
    mockNavigate.mockReset();
  });
});

describe('useGoogleComplete — onAccountCreated', () => {
  it('runs once the server created the account, before the success path navigates', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleComplete).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });
    const onAccountCreated = vi.fn(() => {
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    const { useGoogleComplete } = await import('./useGoogleComplete');
    const { result } = renderHook(() => useGoogleComplete({ onAccountCreated }), {
      wrapper: createWrapper(['/register/google']),
    });

    result.current.mutate(PAYLOAD);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(onAccountCreated).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
  });

  it('does not run when the server refused', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleComplete).mockRejectedValueOnce(await makeConsumedHttpError(409, {}));
    const onAccountCreated = vi.fn();

    const { useGoogleComplete } = await import('./useGoogleComplete');
    const { result } = renderHook(() => useGoogleComplete({ onAccountCreated }), {
      wrapper: createWrapper(['/register/google']),
    });

    result.current.mutate(PAYLOAD);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(onAccountCreated).not.toHaveBeenCalled();
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleComplete).mockReset();
    mockNavigate.mockReset();
  });
});
