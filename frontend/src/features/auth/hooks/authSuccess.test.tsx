import { renderHook } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { makeUser } from '@/test/factories';

const mockNavigate = vi.fn();

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock('sonner', () => ({ toast: { dismiss: vi.fn() } }));

vi.mock('@/shared/stores', () => ({
  useStore: () => ({ setUser: vi.fn(), setCsrfToken: vi.fn() }),
}));

const { useAuthSuccessHandler } = await import('./authSuccess');

function login(entry: string, role: 'owner' | 'superadmin' = 'owner') {
  const { result } = renderHook(() => useAuthSuccessHandler(), {
    wrapper: ({ children }) => <MemoryRouter initialEntries={[entry]}>{children}</MemoryRouter>,
  });
  result.current({ user: makeUser({ role }), csrf_token: 'csrf' });
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
    expect(mockNavigate).toHaveBeenCalledWith('/complexes', { replace: true });
  });

  it('ignores a protocol-relative destination', () => {
    login('/login?from=%2F%2Fevil.example');
    expect(mockNavigate).toHaveBeenCalledWith('/complexes', { replace: true });
  });

  it('sends a superadmin to /admin when there is no from at all', () => {
    login('/login', 'superadmin');
    expect(mockNavigate).toHaveBeenCalledWith('/admin', { replace: true });
  });
});
