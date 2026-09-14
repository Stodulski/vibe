import { StrictMode, type ReactNode } from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { createWrapper } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';

vi.mock('../api/auth.api', () => ({
  authApi: {
    googleExchange: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), dismiss: vi.fn() } }));

const mockSetCsrfToken = vi.fn();
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

const CSRF_COOKIE_VALUE = 'a-csrf-cookie';

function setCsrfCookie() {
  document.cookie = `g_csrf_token=${CSRF_COOKIE_VALUE}`;
}

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

async function renderExchange(code: string | null, strict = false) {
  const { useGoogleExchange } = await import('./useGoogleExchange');
  const Base = createWrapper(['/auth/google/return']);
  const wrapper = strict
    ? ({ children }: { children: ReactNode }) => (
        <StrictMode>
          <Base>{children}</Base>
        </StrictMode>
      )
    : Base;

  return renderHook(
    () => {
      useGoogleExchange(code);
    },
    { wrapper },
  );
}

afterEach(async () => {
  const { authApi } = await import('../api/auth.api');
  vi.mocked(authApi.googleExchange).mockReset();
  mockNavigate.mockReset();
  document.cookie = 'g_csrf_token=; max-age=0';
});

describe('useGoogleExchange — success', () => {
  it('logs in exactly like the popup exchange did when the account already exists', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    setCsrfCookie();

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockSetCsrfToken).toHaveBeenCalledWith('token');
    });
    expect(authApi.googleExchange).toHaveBeenCalledWith({ code: 'a-code', g_csrf_token: CSRF_COOKIE_VALUE });
    expect(mockSetSessionUser).toHaveBeenCalledWith(expect.objectContaining({ id: '1' }));
    expect(mockNavigate).toHaveBeenCalledWith('/complexes', { replace: true });
  });

  it('hands off to /register/google with the profile in router state when the email is unknown', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockResolvedValueOnce({
      needs_profile: true,
      profile_token: 'a-profile-token',
      profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
    });

    setCsrfCookie();

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/register/google', {
        state: {
          profile_token: 'a-profile-token',
          profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
        },
        replace: true,
      });
    });
    expect(mockSetSessionUser).not.toHaveBeenCalled();
  });
});

describe('useGoogleExchange — failure', () => {
  // The code is single-use and lives 120 s; a reload of this page, or a
  // second tab, spends one that is already gone. That is a 422, and it reads
  // as "expired", not as "Google is down".
  it('sends an invalid or expired code back to /login?error=google_expired', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockRejectedValueOnce(
      await makeConsumedHttpError(422, {
        type: 'https://vibe.com.ar/problems/validation',
        errors: [{ field: 'code', message: 'invalid or expired' }],
      }),
    );

    setCsrfCookie();

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_expired', { replace: true });
    });
  });

  // A dropped connection carries no problem body at all, so nothing can be
  // said about the code itself — only that the exchange could not happen.
  it('sends a network failure back to /login?error=google_unavailable', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockRejectedValueOnce(new TypeError('Failed to fetch'));

    setCsrfCookie();

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_unavailable', { replace: true });
    });
  });

  it('sends any other server failure back to /login?error=google_unavailable', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockRejectedValueOnce(await makeConsumedHttpError(500, {}));

    setCsrfCookie();

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_unavailable', { replace: true });
    });
  });

  // Without the cookie there is no double submit to make, so the backend
  // would refuse the request with the same 422 an unknown code gets. Failing
  // closed here says the same thing without spending the code — and it is
  // also what a browser with cookies blocked looks like.
  it('never calls the API and reports an expired sign-in when the cookie is missing', async () => {
    const { authApi } = await import('../api/auth.api');

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_expired', { replace: true });
    });
    expect(authApi.googleExchange).not.toHaveBeenCalled();
  });

  // The cookie is set here on purpose: it is the missing *code* this test is
  // about, and the two exits say different things.
  it('never calls the API and reports a rejected sign-in when there is no code', async () => {
    const { authApi } = await import('../api/auth.api');
    setCsrfCookie();

    await renderExchange(null);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_rejected', { replace: true });
    });
    expect(authApi.googleExchange).not.toHaveBeenCalled();
  });
});

describe('useGoogleExchange — single use', () => {
  // StrictMode mounts, unmounts and remounts every effect on purpose. A code
  // that is spent twice is a sign-in that fails on its own second request, so
  // the guard is a ref rather than a mutation flag: it is the same object
  // across that double invocation.
  it('sends exactly one request under a StrictMode double mount', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockResolvedValue({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    setCsrfCookie();

    const { rerender } = await renderExchange('a-code', true);
    rerender();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalled();
    });
    expect(authApi.googleExchange).toHaveBeenCalledTimes(1);
  });
});
