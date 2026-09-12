import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { createWrapper } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('../api/auth.api', () => ({
  authApi: {
    googleSignIn: vi.fn(),
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

async function triggerGoogleSignInError(backendError: unknown) {
  const { authApi } = await import('../api/auth.api');
  vi.mocked(authApi.googleSignIn).mockRejectedValueOnce(backendError);

  const { useGoogleSignIn } = await import('./useGoogleSignIn');
  const { result } = renderHook(() => useGoogleSignIn(), { wrapper: createWrapper(['/login']) });

  result.current.mutate('a-credential');

  await waitFor(() => {
    expect(result.current.isError).toBe(true);
  });
}

describe('useGoogleSignIn — onError', () => {
  it('shows the rate-limit message on a 429', async () => {
    await triggerGoogleSignInError(await makeConsumedHttpError(429, {}));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.rateLimitError);
  });

  it('shows the googleUnavailable message on a 503 instead of the raw server sentence', async () => {
    await triggerGoogleSignInError(await makeConsumedHttpError(503, { error: 'google sign-in is not configured' }));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.googleUnavailable);
  });

  it('shows the generic invalid-credentials message on a 401 (inactive/locked account)', async () => {
    await triggerGoogleSignInError(await makeConsumedHttpError(401, {}));
    // No "check your inbox" hint here, unlike useLogin: Google already
    // verified the address, so a 401 on this path is never about verification.
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.invalidCredentials);
  });

  it('falls back to the generic Google sign-in error on anything else (e.g. 422)', async () => {
    await triggerGoogleSignInError(await makeConsumedHttpError(422, {}));
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.googleSignInError);
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleSignIn).mockReset();
    mockNavigate.mockReset();
  });
});

describe('useGoogleSignIn — onSuccess', () => {
  it('logs in exactly like useLogin when the account already exists', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleSignIn).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    const { useGoogleSignIn } = await import('./useGoogleSignIn');
    const { result } = renderHook(() => useGoogleSignIn(), { wrapper: createWrapper(['/login']) });

    result.current.mutate('a-credential');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockSetCsrfToken).toHaveBeenCalledWith('token');
    expect(mockSetSessionUser).toHaveBeenCalledWith(expect.objectContaining({ id: '1' }));
    expect(mockNavigate).toHaveBeenCalledWith('/complexes', { replace: true });
  });

  it('navigates to /register/google carrying profile_token and profile in state when the email is unknown', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleSignIn).mockResolvedValueOnce({
      needs_profile: true,
      profile_token: 'a-profile-token',
      profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
    });

    const { useGoogleSignIn } = await import('./useGoogleSignIn');
    const { result } = renderHook(() => useGoogleSignIn(), { wrapper: createWrapper(['/login']) });

    result.current.mutate('a-credential');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockNavigate).toHaveBeenCalledWith('/register/google', {
      state: {
        profile_token: 'a-profile-token',
        profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
      },
    });
    expect(mockSetSessionUser).not.toHaveBeenCalled();
  });

  afterEach(async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleSignIn).mockReset();
    mockNavigate.mockReset();
  });
});
