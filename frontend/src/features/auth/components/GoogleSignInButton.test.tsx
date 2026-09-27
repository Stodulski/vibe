import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import { GoogleSignInButton } from './GoogleSignInButton';

const mockEnv = vi.hoisted((): { VITE_GOOGLE_CLIENT_ID: string | undefined; VITE_API_URL: string } => ({
  VITE_GOOGLE_CLIENT_ID: 'test-client-id',
  VITE_API_URL: '/api/v1',
}));
vi.mock('@/shared/lib/env', () => ({ env: mockEnv }));

/**
 * The button reads the destination out of the current location (router state
 * or `?from=`), so every case here needs a real router around it.
 */
function renderButton(entry: string | { pathname: string; state?: unknown } = '/login') {
  const routes: RouteObject[] = [{ path: '*', element: <GoogleSignInButton /> }];
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  return render(<RouterProvider router={router} />);
}

describe('GoogleSignInButton — rendering', () => {
  beforeEach(() => {
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';
    mockEnv.VITE_API_URL = '/api/v1';
  });

  it('renders nothing when Google sign-in is not offered on this deployment', () => {
    mockEnv.VITE_GOOGLE_CLIENT_ID = undefined;

    const { container } = renderButton();

    expect(container).toBeEmptyDOMElement();
  });

  // No JS in the way: `/auth/google/start` sets a cookie and 302s, which
  // only happens on a real top-level navigation, never a fetch or an
  // onClick — see the component.
  it('links straight to the backend OIDC start endpoint', () => {
    renderButton();

    const link = screen.getByRole('link', { name: ES_AR.auth.continueWithGoogle });
    expect(link).toHaveAttribute('href', '/api/v1/auth/google/start');
  });

  it('is shaped like the primary button', () => {
    renderButton();

    const link = screen.getByRole('link', { name: ES_AR.auth.continueWithGoogle });
    expect(link).toHaveClass('h-11', 'w-full', 'rounded-full');
  });
});

// The redirect hop leaves this page entirely and comes back on
// `/auth/google/callback?code=…&state=…`, which knows nothing about where
// the person was going. Parking it here, while `/login` still knows, is the
// whole mechanism — see `rememberGoogleReturnPath`.
describe('GoogleSignInButton — remembering where the visitor was going', () => {
  const KEY = 'vibe.google-signin.from';

  beforeEach(() => {
    mockEnv.VITE_GOOGLE_CLIENT_ID = 'test-client-id';
    mockEnv.VITE_API_URL = '/api/v1';
    window.sessionStorage.clear();
  });

  afterEach(() => {
    window.sessionStorage.clear();
  });

  it('remembers the destination ProtectedRoute put in router state', () => {
    renderButton({ pathname: '/login', state: { from: { pathname: '/bookings/abc' } } });

    expect(window.sessionStorage.getItem(KEY)).toBe('/bookings/abc');
  });

  // `?from=` is how the destination survives the hard navigation `ky.ts` does
  // when a session cannot be refreshed.
  it('remembers the destination a hard 401 redirect left in ?from=', () => {
    renderButton('/login?from=%2Fbookings%3Fdate%3D2026-03-18');

    expect(window.sessionStorage.getItem(KEY)).toBe('/bookings?date=2026-03-18');
  });

  it('refuses a destination outside the app', () => {
    renderButton('/login?from=https%3A%2F%2Fevil.example%2Fsteal');

    expect(window.sessionStorage.getItem(KEY)).toBeNull();
  });

  // A destination left over from an abandoned attempt must not be inherited
  // by a later sign-in that had none of its own.
  it('clears a stale destination when this visit carries none', () => {
    window.sessionStorage.setItem(KEY, '/bookings');

    renderButton('/login');

    expect(window.sessionStorage.getItem(KEY)).toBeNull();
  });
});
