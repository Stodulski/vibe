import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import GoogleCallbackPage from './GoogleCallbackPage';

vi.mock('../api/auth.api', () => ({
  authApi: {
    googleFinish: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), dismiss: vi.fn() } }));

afterEach(() => {
  window.sessionStorage.clear();
});

function renderPage(search: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      { path: '/auth/google/callback', element: <GoogleCallbackPage /> },
      { path: '/login', element: <div>Login page</div> },
      { path: '/dashboard', element: <div>Dashboard page</div> },
    ],
    { initialEntries: [`/auth/google/callback${search}`] },
  );
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe('GoogleCallbackPage', () => {
  it('shows an accessible loader while the code and state are being exchanged', async () => {
    const { authApi } = await import('../api/auth.api');
    // Never resolves: this is the only state the page ever renders, every
    // outcome navigates away.
    vi.mocked(authApi.googleFinish).mockReturnValueOnce(new Promise(() => undefined));

    renderPage('?code=an-opaque-code&state=an-opaque-state');

    expect(screen.getByRole('status')).toHaveAccessibleName(ES_AR.auth.googleReturnLoading);
    expect(screen.getByText(ES_AR.auth.googleReturnLoading)).toBeInTheDocument();
    await waitFor(() => {
      expect(authApi.googleFinish).toHaveBeenCalledWith({ code: 'an-opaque-code', state: 'an-opaque-state' });
    });
  });

  it('exchanges the code and state carried in the query string for a session', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockResolvedValueOnce({
      csrf_token: 'token',
      user: { id: '1', email: 'juan@test.com', role: 'owner' } as never,
    });

    renderPage('?code=an-opaque-code&state=an-opaque-state');

    await waitFor(() => {
      expect(screen.getByText('Dashboard page')).toBeInTheDocument();
    });
  });

  // No state means the page was opened by hand or reached by a callback that
  // lost its query — there is nothing to spend, so it reads as expired
  // rather than as an unavailable API.
  it('goes to /login?error=google_expired without calling the API when the state is missing', async () => {
    const { authApi } = await import('../api/auth.api');

    const router = renderPage('?code=an-opaque-code');

    await waitFor(() => {
      expect(screen.getByText('Login page')).toBeInTheDocument();
    });
    expect(router.state.location.search).toBe('?error=google_expired');
    expect(authApi.googleFinish).not.toHaveBeenCalled();
  });

  // The visitor backed out of Google's account chooser — not a failure.
  it('goes quietly to /login when Google reports access_denied', async () => {
    const { authApi } = await import('../api/auth.api');

    const router = renderPage('?error=access_denied&state=an-opaque-state');

    await waitFor(() => {
      expect(screen.getByText('Login page')).toBeInTheDocument();
    });
    expect(router.state.location.search).toBe('');
    expect(authApi.googleFinish).not.toHaveBeenCalled();
  });

  it('goes to /login?error=google_unavailable when the finish call fails', async () => {
    const { authApi } = await import('../api/auth.api');
    vi.mocked(authApi.googleFinish).mockRejectedValueOnce(new TypeError('Failed to fetch'));

    const router = renderPage('?code=an-opaque-code&state=an-opaque-state');

    await waitFor(() => {
      expect(screen.getByText('Login page')).toBeInTheDocument();
    });
    expect(router.state.location.search).toBe('?error=google_unavailable');
  });
});
