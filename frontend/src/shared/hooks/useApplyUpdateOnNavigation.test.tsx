import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, Link, Outlet, RouterProvider } from 'react-router-dom';
import { applyPendingServiceWorkerUpdate, isUpdatePending } from '@/shared/lib/serviceWorkerUpdate';
import { hasUnsavedWork, markUnsavedWork } from '@/shared/lib/unsavedWork';
import { useApplyUpdateOnNavigation } from './useApplyUpdateOnNavigation';

vi.mock('@/shared/lib/serviceWorkerUpdate', () => ({
  applyPendingServiceWorkerUpdate: vi.fn(),
  isUpdatePending: vi.fn(() => false),
}));

const applyUpdate = vi.mocked(applyPendingServiceWorkerUpdate);
const pending = vi.mocked(isUpdatePending);

/** The pathless root the hook is really mounted on — see `ScrollToTop`. */
function Root() {
  useApplyUpdateOnNavigation();
  return <Outlet />;
}

function renderApp() {
  const router = createMemoryRouter(
    [
      {
        element: <Root />,
        children: [
          {
            path: '/bookings',
            element: (
              <div>
                <p>Reservas</p>
                <Link to="/courts">Canchas</Link>
                <Link to="/bookings?tab=historial">Historial</Link>
              </div>
            ),
          },
          { path: '/courts', element: <p>Canchas</p> },
        ],
      },
    ],
    { initialEntries: ['/bookings'] },
  );
  render(<RouterProvider router={router} />);
  return userEvent.setup();
}

// PWA-09: a route change is the one moment a reload costs the person nothing,
// because the screen they were on is being replaced anyway.
describe('useApplyUpdateOnNavigation', () => {
  beforeEach(() => {
    markUnsavedWork('form', false);
    pending.mockReturnValue(false);
  });

  it('applies a pending update when the pathname changes', async () => {
    pending.mockReturnValue(true);
    const user = renderApp();

    // Mounting on a route is not navigating away from anything.
    expect(applyUpdate).not.toHaveBeenCalled();

    await user.click(screen.getByRole('link', { name: 'Canchas' }));

    expect(await screen.findByText('Canchas')).toBeInTheDocument();
    expect(applyUpdate).toHaveBeenCalledTimes(1);
  });

  // A `?tab=` keeps the person on the same screen with the same state — the
  // same rule `useUnsavedChangesBlocker` uses to decide what counts as leaving.
  it('ignores a query-string change on the same pathname', async () => {
    pending.mockReturnValue(true);
    const user = renderApp();

    await user.click(screen.getByRole('link', { name: 'Historial' }));

    expect(screen.getByText('Reservas')).toBeInTheDocument();
    expect(applyUpdate).not.toHaveBeenCalled();
  });

  it('leaves the update pending while a form holds unsaved changes', async () => {
    pending.mockReturnValue(true);
    markUnsavedWork('form', true);
    const user = renderApp();

    await user.click(screen.getByRole('link', { name: 'Canchas' }));

    expect(await screen.findByText('Canchas')).toBeInTheDocument();
    expect(hasUnsavedWork()).toBe(true);
    expect(applyUpdate).not.toHaveBeenCalled();
  });

  it('does nothing on navigation when no new build is waiting', async () => {
    const user = renderApp();

    await user.click(screen.getByRole('link', { name: 'Canchas' }));

    expect(await screen.findByText('Canchas')).toBeInTheDocument();
    expect(applyUpdate).not.toHaveBeenCalled();
  });
});
