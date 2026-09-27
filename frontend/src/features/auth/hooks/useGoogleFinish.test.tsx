import { StrictMode, type ReactNode } from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { createWrapper } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';

vi.mock('../api/auth.api', () => ({
  authApi: {
    googleFinish: vi.fn(),
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

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

async function renderFinish(code: string | null, state: string | null, error: string | null = null, strict = false) {
  const { useGoogleFinish } = await import('./useGoogleFinish');
  const Base = createWrapper(['/auth/google/callback']);
  const wrapper = strict
    ? ({ children }: { children: ReactNode }) => (
        <StrictMode>
          <Base>{children}</Base>
        </StrictMode>
      )
    : Base;

  return renderHook(
    () => {
      useGoogleFinish(code, state, error);
    },
    { wrapper },
  );
}

afterEach(async () => {
  const { authApi } = await import('../api/auth.api');
  vi.mocked(authApi.googleFinish).mockReset();
  mockNavigate.mockReset();
  mockAuthSuccess.mockReset();
  window.sessionStorage.clear();
});

describe('useGoogleFinish — success', () => {
  it('logs in exactly like the old exchange did when the account already exists', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockSetCsrfToken).toHaveBeenCalledWith('token');
    });
    expect(authApi.googleFinish).toHaveBeenCalledWith({ code: 'a-code', state: 'a-state' });
    expect(mockSetSessionUser).toHaveBeenCalledWith(expect.objectContaining({ id: '1' }));
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
  });

  it('hands off to /register/google with the profile in router state when the email is unknown', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockResolvedValueOnce({
      needs_profile: true,
      profile_token: 'a-profile-token',
      profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
    });

    await renderFinish('a-code', 'a-state');

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

describe('useGoogleFinish — Google refused before a code ever arrived', () => {
  // The visitor backed out of the account chooser — not a failure, so it
  // returns to a plain /login with no message.
  it('returns quietly to /login on access_denied', async () => {
    const { authApi } = await import('../api/auth.api');

    await renderFinish(null, null, 'access_denied');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login', { replace: true });
    });
    expect(authApi.googleFinish).not.toHaveBeenCalled();
  });

  it('sends any other OAuth error back to /login?error=google_unavailable', async () => {
    const { authApi } = await import('../api/auth.api');

    await renderFinish(null, null, 'server_error');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_unavailable', { replace: true });
    });
    expect(authApi.googleFinish).not.toHaveBeenCalled();
  });
});

describe('useGoogleFinish — a callback that lost its query', () => {
  it('reports an expired sign-in and never calls the API when the code is missing', async () => {
    const { authApi } = await import('../api/auth.api');

    await renderFinish(null, 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_expired', { replace: true });
    });
    expect(authApi.googleFinish).not.toHaveBeenCalled();
  });

  it('reports an expired sign-in and never calls the API when the state is missing', async () => {
    const { authApi } = await import('../api/auth.api');

    await renderFinish('a-code', null);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_expired', { replace: true });
    });
    expect(authApi.googleFinish).not.toHaveBeenCalled();
  });
});

describe('useGoogleFinish — /finish failure', () => {
  // A missing, mismatched, expired or replayed state, or Google's own
  // invalid_grant: the backend names the `code` field either way.
  it('sends a 422 naming `code` back to /login?error=google_expired', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockRejectedValueOnce(
      await makeConsumedHttpError(422, {
        type: 'https://vibe.com.ar/problems/validation',
        errors: [{ field: 'code', message: 'invalid or expired' }],
      }),
    );

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_expired', { replace: true });
    });
  });

  // The ID token or its nonce was rejected — a different failure from an
  // expired state, and the copy says so.
  it('sends a 422 naming `credential` back to /login?error=google_rejected', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockRejectedValueOnce(
      await makeConsumedHttpError(422, {
        type: 'https://vibe.com.ar/problems/validation',
        errors: [{ field: 'credential', message: 'invalid token' }],
      }),
    );

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_rejected', { replace: true });
    });
  });

  // A dropped connection carries no problem body at all, so nothing can be
  // said about the code itself — only that the finish call could not happen.
  it('sends a network failure back to /login?error=google_unavailable', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockRejectedValueOnce(new TypeError('Failed to fetch'));

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_unavailable', { replace: true });
    });
  });

  // A 429 from the finish call itself means the sign-in was throttled, not
  // that Google is unreachable — a distinct message from google_unavailable.
  it('sends a 429 back to /login?error=google_rate_limited', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockRejectedValueOnce(await makeConsumedHttpError(429, {}));

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_rate_limited', { replace: true });
    });
  });

  it('sends any other server failure back to /login?error=google_unavailable', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockRejectedValueOnce(await makeConsumedHttpError(500, {}));

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login?error=google_unavailable', { replace: true });
    });
  });
});

describe('useGoogleFinish — single use', () => {
  // StrictMode mounts, unmounts and remounts every effect on purpose. A code
  // and state spent twice is a sign-in that fails on its own second request,
  // so the guard is a ref rather than a mutation flag: it is the same object
  // across that double invocation.
  it('sends exactly one request under a StrictMode double mount', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockResolvedValue({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    const { rerender } = await renderFinish('a-code', 'a-state', null, true);
    rerender();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalled();
    });
    expect(authApi.googleFinish).toHaveBeenCalledTimes(1);
  });
});

// `/auth/google/callback?code=…&state=…` knows nothing about where the
// visitor was heading — the button parked it in sessionStorage before
// leaving for Google.
describe('useGoogleFinish — the destination parked before the redirect', () => {
  const KEY = 'vibe.google-signin.from';

  it('returns to the remembered page instead of the role default', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });
    window.sessionStorage.setItem(KEY, '/bookings/abc');

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/bookings/abc', { replace: true });
    });
    expect(mockAuthSuccess).toHaveBeenCalledWith({ from: '/bookings/abc' });
    // Spent, like the code and state it travelled with.
    expect(window.sessionStorage.getItem(KEY)).toBeNull();
  });

  it('falls back to the role default when nothing was remembered', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    await renderFinish('a-code', 'a-state');

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
    });
    expect(mockAuthSuccess).toHaveBeenCalledWith({ from: undefined });
  });
});

// A sibling describe, not nested: max-lines-per-function counts a describe
// callback's whole body.
describe('useGoogleFinish — that destination across the profile step', () => {
  const KEY = 'vibe.google-signin.from';

  // The profile step is one more hop before there is a session, so the
  // destination rides in router state — `useGoogleComplete` finishes through
  // the same handler, which reads `from` straight out of `location.state`.
  it('carries the destination into /register/google router state', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockResolvedValueOnce({
      needs_profile: true,
      profile_token: 'a-profile-token',
      profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
    });
    window.sessionStorage.setItem(KEY, '/bookings');

    await renderFinish('a-code', 'a-state');

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
    vi.mocked(authApi.googleFinish).mockResolvedValueOnce({
      needs_profile: true,
      profile_token: 'a-profile-token',
      profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
    });

    await renderFinish('a-code', 'a-state');

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
