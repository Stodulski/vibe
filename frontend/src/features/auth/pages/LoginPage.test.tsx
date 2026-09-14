import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import LoginPage from './LoginPage';

// The form itself is covered by LoginForm.test.tsx; this suite is only about
// the banner the page shows for a Google sign-in that came back refused.
vi.mock('@/features/auth', () => ({
  LoginForm: () => <div>Login form</div>,
}));

function renderPage(search: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter([{ path: '/login', element: <LoginPage /> }], {
    initialEntries: [`/login${search}`],
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe('LoginPage — Google error banner', () => {
  it.each([
    ['google_rejected', ES_AR.auth.googleErrorRejected],
    ['google_unavailable', ES_AR.auth.googleErrorUnavailable],
    ['google_expired', ES_AR.auth.googleErrorExpired],
  ])('shows the inline message for ?error=%s', async (value, message) => {
    renderPage(`?error=${value}`);

    expect(await screen.findByRole('alert')).toHaveTextContent(message);
  });

  // The message is copied into state before the parameter goes, so the line
  // survives the re-render its own removal causes — and a reload, or a link
  // shared out of the address bar, no longer resurrects the failure.
  it('clears ?error= from the URL without losing the message', async () => {
    const router = renderPage('?error=google_rejected');

    await waitFor(() => {
      expect(router.state.location.search).toBe('');
    });
    expect(screen.getByRole('alert')).toHaveTextContent(ES_AR.auth.googleErrorRejected);
  });

  // `?from=` is the page a ProtectedRoute sent the visitor away from;
  // `useAuthSuccessHandler` still reads it after a successful login.
  it('keeps every other query parameter when it clears ?error=', async () => {
    const router = renderPage('?error=google_expired&from=%2Fbookings');

    await waitFor(() => {
      expect(router.state.location.search).toBe('?from=%2Fbookings');
    });
  });

  it('ignores an unknown error value and clears it anyway', async () => {
    const router = renderPage('?error=something_else');

    await waitFor(() => {
      expect(router.state.location.search).toBe('');
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('shows no banner at all on a plain visit', () => {
    renderPage('');

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByText('Login form')).toBeInTheDocument();
  });
});
