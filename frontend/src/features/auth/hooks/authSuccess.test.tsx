import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { makeUser } from '@/test/factories';

const mockNavigate = vi.fn();

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock('sonner', () => ({ toast: { dismiss: vi.fn() } }));

// Selector-aware, like the real store: the handler reads `setCsrfToken` as its
// own atomic slice (STORE-02); the user goes to the query cache (DATA-11).
vi.mock('@/shared/stores', () => ({
  useStore: (selector: (s: { setCsrfToken: () => void }) => unknown) => selector({ setCsrfToken: vi.fn() }),
}));

const { useAuthSuccessHandler } = await import('./authSuccess');

function login(entry: string, role: 'owner' | 'superadmin' = 'owner', options?: { from?: string | undefined }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const { result } = renderHook(() => useAuthSuccessHandler(), {
    wrapper: ({ children }) => (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[entry]}>{children}</MemoryRouter>
      </QueryClientProvider>
    ),
  });
  result.current({ user: makeUser({ role }), csrf_token: 'csrf' }, options);
  return queryClient;
}

/**
 * `ProtectedRoute` sends the intended destination in router state; the 401
 * handler in `src/shared/lib/ky.ts` cannot, because it ends the session with
 * `window.location.href` and router state does not survive a document load.
 * It sends `?from=` instead, and both have to be honoured — and both have to
 * go through the same allowlist, or the query parameter becomes an open
 * redirect anyone can put in a link.
 */
describe('useAuthSuccessHandler — the ?from= a hard 401 redirect leaves behind', () => {
  beforeEach(() => {
    mockNavigate.mockClear();
  });

  it('returns the person to the page the expired session interrupted', () => {
    login('/login?from=%2Fbookings');
    expect(mockNavigate).toHaveBeenCalledWith('/bookings', { replace: true });
  });

  it('keeps the query string that was part of that page', () => {
    login('/login?from=%2Fbookings%3Fdate%3D2026-03-18');
    expect(mockNavigate).toHaveBeenCalledWith('/bookings?date=2026-03-18', { replace: true });
  });

  it('ignores a destination outside the app and uses the role default instead', () => {
    login('/login?from=https%3A%2F%2Fevil.example%2Fsteal');
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
  });

  it('ignores a protocol-relative destination', () => {
    login('/login?from=%2F%2Fevil.example');
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
  });

  it('sends a superadmin to /admin when there is no from at all', () => {
    login('/login', 'superadmin');
    expect(mockNavigate).toHaveBeenCalledWith('/admin', { replace: true });
  });
});

/**
 * The Google redirect flow signs in on `/auth/google/return`, a page the
 * visitor never chose: it carries neither the router state nor the `?from=`
 * the original `/login` had. `useGoogleExchange` therefore hands the
 * destination over explicitly, and it has to beat what the location says —
 * without becoming a way past the allowlist.
 */
describe('useAuthSuccessHandler — an explicit destination from the caller', () => {
  beforeEach(() => {
    mockNavigate.mockClear();
  });

  it('uses the destination the caller passed', () => {
    login('/auth/google/return?code=abc', 'owner', { from: '/bookings/abc' });
    expect(mockNavigate).toHaveBeenCalledWith('/bookings/abc', { replace: true });
  });

  it('wins over the ?from= on the current location', () => {
    login('/login?from=%2Fclients', 'owner', { from: '/bookings' });
    expect(mockNavigate).toHaveBeenCalledWith('/bookings', { replace: true });
  });

  // An override is a convenience for a page that lost its context, not an
  // exemption: it goes through exactly the same allowlist.
  it('is filtered like every other destination', () => {
    login('/auth/google/return?code=abc', 'owner', { from: 'https://evil.example/steal' });
    expect(mockNavigate).toHaveBeenCalledWith('/dashboard', { replace: true });
  });

  it('falls back to the location when the caller passes nothing', () => {
    login('/login?from=%2Fclients', 'owner', { from: undefined });
    expect(mockNavigate).toHaveBeenCalledWith('/clients', { replace: true });
  });
});
