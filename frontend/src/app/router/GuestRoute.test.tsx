import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

vi.mock('@/features/auth/hooks/useAuth', () => ({
  useAuth: vi.fn(),
}));

import { useAuth } from '@/features/auth/hooks/useAuth';
import { GuestRoute } from '@/app/router/GuestRoute';
import { setSessionHint } from '@/shared/lib/sessionHint';
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
function guardedTree() {
  return (
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
    </MemoryRouter>
  );
}

function renderGuarded() {
  return render(guardedTree());
}

describe('GuestRoute', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('shows the page to a visitor who is not signed in', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: null,
      pendingEmail: null,
      isAuthenticated: false,
      isLoading: false,
    });

    renderGuarded();

    expect(screen.getByTestId('login-form')).toBeInTheDocument();
  });

  it('sends a signed-in user away from the sign-in page', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: makeUser({ role: 'owner' }),
      pendingEmail: null,
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
      pendingEmail: null,
      isAuthenticated: true,
      isLoading: false,
    });

    renderGuarded();

    expect(screen.queryByTestId('login-form')).not.toBeInTheDocument();
  });
});

// Each case here is a page load in a browser that does or does not hold a
// session, while the check is still running. A sibling describe, not nested in
// the one above: max-lines-per-function counts a describe's whole body.
describe('GuestRoute while the session check runs', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  // A browser that has signed in before may hold a session, so the guard waits
  // for the check rather than flashing the form at someone who is signed in.
  it('waits for the check in a browser that has signed in before', () => {
    setSessionHint(true);
    vi.mocked(useAuth).mockReturnValue({
      user: null,
      pendingEmail: null,
      isAuthenticated: false,
      isLoading: true,
    });

    renderGuarded();

    expect(screen.queryByTestId('login-form')).not.toBeInTheDocument();
    expect(screen.queryByTestId('home')).not.toBeInTheDocument();
    // The loading state must be announced to screen readers, not a bare spinner icon.
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  // A first-time visitor has no session to find, so the form does not wait on
  // the check: it is the page they came for, and the check only decides whether
  // to send them on.
  it('shows the form at once while the check runs in a browser that has never signed in', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: null,
      pendingEmail: null,
      isAuthenticated: false,
      isLoading: true,
    });

    renderGuarded();

    expect(screen.getByTestId('login-form')).toBeInTheDocument();
    expect(screen.queryByTestId('home')).not.toBeInTheDocument();
  });

  it('sends the visitor on when the check finds a session after the form was shown', () => {
    vi.mocked(useAuth).mockReturnValue({
      user: null,
      pendingEmail: null,
      isAuthenticated: false,
      isLoading: true,
    });
    const view = renderGuarded();
    expect(screen.getByTestId('login-form')).toBeInTheDocument();

    vi.mocked(useAuth).mockReturnValue({
      user: makeUser({ role: 'owner' }),
      pendingEmail: null,
      isAuthenticated: true,
      isLoading: false,
    });
    view.rerender(guardedTree());

    expect(screen.queryByTestId('login-form')).not.toBeInTheDocument();
    expect(screen.getByTestId('home')).toBeInTheDocument();
  });
});
