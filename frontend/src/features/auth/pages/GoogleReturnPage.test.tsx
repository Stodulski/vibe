import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import GoogleReturnPage from './GoogleReturnPage';

vi.mock('../api/auth.api', () => ({
  authApi: {
    googleExchange: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), dismiss: vi.fn() } }));

const CSRF_COOKIE_VALUE = 'a-csrf-cookie';

// Google sets this on the app origin during the redirect hop, non-HttpOnly so
// the page can read it back and double-submit it with the code.
function setCsrfCookie() {
  document.cookie = `g_csrf_token=${CSRF_COOKIE_VALUE}`;
}

afterEach(() => {
  document.cookie = 'g_csrf_token=; max-age=0';
});

function renderPage(search: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      { path: '/auth/google/return', element: <GoogleReturnPage /> },
      { path: '/login', element: <div>Login page</div> },
      { path: '/complexes', element: <div>Complexes page</div> },
    ],
    { initialEntries: [`/auth/google/return${search}`] },
  );
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe('GoogleReturnPage', () => {
  it('shows an accessible loader while the code is being exchanged', async () => {
    const { authApi } = await import('../api/auth.api');
    // Never resolves: this is the only state the page ever renders, every
    // outcome navigates away.
    vi.mocked(authApi.googleExchange).mockReturnValueOnce(new Promise(() => undefined));

    setCsrfCookie();

    renderPage('?code=an-opaque-code');

    expect(screen.getByRole('status')).toHaveAccessibleName(ES_AR.auth.googleReturnLoading);
    expect(screen.getByText(ES_AR.auth.googleReturnLoading)).toBeInTheDocument();
    await waitFor(() => {
      expect(authApi.googleExchange).toHaveBeenCalledWith({
        code: 'an-opaque-code',
        g_csrf_token: CSRF_COOKIE_VALUE,
      });
    });
  });

  it('exchanges the code carried in the query string for a session', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });
    setCsrfCookie();

    renderPage('?code=an-opaque-code');

    await waitFor(() => {
      expect(screen.getByText('Complexes page')).toBeInTheDocument();
    });
  });

  // No code means the page was opened by hand or reached by a redirect that
  // lost its query — there is nothing to spend, so it is a rejected sign-in
  // rather than an unavailable API.
  it('goes to /login?error=google_rejected without calling the API when the code is missing', async () => {
    const { authApi } = await import('../api/auth.api');
    setCsrfCookie();

    const router = renderPage('');

    await waitFor(() => {
      expect(screen.getByText('Login page')).toBeInTheDocument();
    });
    expect(router.state.location.search).toBe('?error=google_rejected');
    expect(authApi.googleExchange).not.toHaveBeenCalled();
  });

  // Cookies blocked, or the cookie already gone: nothing is worth sending,
  // and the person sees the same sentence a spent code gets.
  it('goes to /login?error=google_expired without calling the API when the cookie is missing', async () => {
    const { authApi } = await import('../api/auth.api');

    const router = renderPage('?code=an-opaque-code');

    await waitFor(() => {
      expect(screen.getByText('Login page')).toBeInTheDocument();
    });
    expect(router.state.location.search).toBe('?error=google_expired');
    expect(authApi.googleExchange).not.toHaveBeenCalled();
  });

  it('goes to /login?error=google_unavailable when the exchange fails', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleExchange).mockRejectedValueOnce(new TypeError('Failed to fetch'));
    setCsrfCookie();

    const router = renderPage('?code=an-opaque-code');

    await waitFor(() => {
      expect(screen.getByText('Login page')).toBeInTheDocument();
    });
    expect(router.state.location.search).toBe('?error=google_unavailable');
  });
});
