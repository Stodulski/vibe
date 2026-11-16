import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider, useLocation, useParams } from 'react-router-dom';
import { eventRedirectRoutes } from './eventRoutes';

function Probe() {
  const { pathname, search } = useLocation();
  const { tournamentId } = useParams();
  return <p data-testid="at">{`${pathname}${search}|${tournamentId ?? ''}`}</p>;
}

function renderAt(url: string) {
  const router = createMemoryRouter(
    [
      ...eventRedirectRoutes,
      { path: '/events', element: <Probe /> },
      { path: '/events/tournaments/:tournamentId', element: <Probe /> },
    ],
    { initialEntries: [url] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

describe('event redirects', () => {
  it.each([
    ['/events/open-play', '/events|'],
    ['/events/open-play?event=ev1', '/events?event=ev1|'],
    ['/events/tournaments', '/events|'],
    ['/open-play', '/events|'],
    ['/open-play?event=ev1', '/events?event=ev1|'],
    ['/tournaments', '/events|'],
    ['/tournaments/t1', '/events/tournaments/t1|t1'],
  ])('sends %s to its new address', (from, to) => {
    renderAt(from);
    expect(screen.getByTestId('at')).toHaveTextContent(to);
  });

  it('leaves the events page itself where it is, query included', () => {
    renderAt('/events?event=ev1');
    expect(screen.getByTestId('at')).toHaveTextContent('/events?event=ev1|');
  });

  it('replaces the old entry, so Back does not bounce through the redirect', () => {
    const router = renderAt('/open-play?event=ev1');
    expect(router.state.historyAction).toBe('REPLACE');
  });
});
