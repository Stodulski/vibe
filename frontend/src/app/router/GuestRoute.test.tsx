import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

vi.mock('@/features/auth/hooks/useAuth', () => ({
  useAuth: vi.fn(),
}));

import { useAuth } from '@/features/auth/hooks/useAuth';
import { GuestRoute } from '@/app/router/GuestRoute';
import { makeUser } from '@/test/factories';

/**
 * GuestRoute keeps a signed-in user off the sign-in and sign-up pages.
 *
 * The guard lives at the route level on purpose. It used to be a redirect
 * inside LoginPage and RegisterPage, which produced an infinite reload loop
 * with ky's 401 handler — see "prevent infinite page reload on login/register
 * when authenticated". Moving it here fixed that, but the behaviour arrived
 * with no test of its own; these are it.
 */
function renderGuarded() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <Routes>
        <Route
          path="/login"
          element={
            <GuestRoute>
              <div data-testid="login-form">LoginForm</div>
            </GuestRoute>
          }
        />
        <Route path="/" element={<div data-testid="home">Home</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('GuestRoute', () => {
  it('shows the page to a visitor who is not signed in', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    renderGuarded();

    expect(screen.getByTestId('login-form')).toBeInTheDocument();
  });

  it('sends a signed-in user away from the sign-in page', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: makeUser({ role: 'owner' }),
      isAuthenticated: true,
      isLoading: false,
    });

    renderGuarded();

    expect(screen.queryByTestId('login-form')).not.toBeInTheDocument();
    expect(screen.getByTestId('home')).toBeInTheDocument();
  });

  it('sends a signed-in superadmin away too', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: makeUser({ role: 'superadmin' }),
      isAuthenticated: true,
      isLoading: false,
    });

    renderGuarded();

    expect(screen.queryByTestId('login-form')).not.toBeInTheDocument();
  });

  // While the session is still resolving the guard must decide nothing.
  // Rendering the form here would flash it at a user who is already signed in;
  // redirecting would bounce a visitor who is not.
  it('waits rather than guessing while the session is loading', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: true,
    });

    renderGuarded();

    expect(screen.queryByTestId('login-form')).not.toBeInTheDocument();
    expect(screen.queryByTestId('home')).not.toBeInTheDocument();
    // The loading state must be announced to screen readers, not a bare spinner icon.
    expect(screen.getByRole('status')).toBeInTheDocument();
  });
});
