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

// Spies on the second argument without mocking the handler away: the real one
// still runs, so the navigation it performs is asserted as before.
const mockAuthSuccess = vi.fn();
vi.mock('./authSuccess', async () => {
  const actual = await vi.importActual<typeof import('./authSuccess')>('./authSuccess');
  return {
    ...actual,
    useAuthSuccessHandler: () => {
      const handle = actual.useAuthSuccessHandler();
      return (data: Parameters<typeof handle>[0], options?: Parameters<typeof handle>[1]) => {
        mockAuthSuccess(options);
        handle(data, options);
      };
    },
  };
});

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
  mockAuthSuccess.mockReset();
  document.cookie = 'g_csrf_token=; max-age=0';
  window.sessionStorage.clear();
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
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
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

// `/auth/google/return?code=…` knows nothing about where the visitor was
// heading — the button parked it in sessionStorage before leaving for Google.
describe('useGoogleExchange — the destination parked before the redirect', () => {
  const KEY = 'vibe.google-signin.from';

  it('returns to the remembered page instead of the role default', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });
    setCsrfCookie();
    window.sessionStorage.setItem(KEY, '/bookings/abc');

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/bookings/abc', { replace: true });
    });
    expect(mockAuthSuccess).toHaveBeenCalledWith({ from: '/bookings/abc' });
    // Spent, like the code it travelled with.
    expect(window.sessionStorage.getItem(KEY)).toBeNull();
  });

  it('falls back to the role default when nothing was remembered', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });
    setCsrfCookie();

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
    });
    expect(mockAuthSuccess).toHaveBeenCalledWith({ from: undefined });
  });
});

// A sibling describe, not nested: max-lines-per-function counts a describe
// callback's whole body.
describe('useGoogleExchange — that destination across the profile step', () => {
  const KEY = 'vibe.google-signin.from';

  // The profile step is one more hop before there is a session, so the
  // destination rides in router state — `useGoogleComplete` finishes through
  // the same handler, which reads `from` straight out of `location.state`.
  it('carries the destination into /register/google router state', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockResolvedValueOnce({
      needs_profile: true,
      profile_token: 'a-profile-token',
      profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
    });
    setCsrfCookie();
    window.sessionStorage.setItem(KEY, '/bookings');

    await renderExchange('a-code');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/register/google', {
        state: {
          profile_token: 'a-profile-token',
          profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
          from: { pathname: '/bookings' },
        },
        replace: true,
      });
    });
  });

  it('leaves /register/google state as it was when there is nothing to carry', async () => {
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
  });
});
