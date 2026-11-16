import type { RouteObject } from 'react-router-dom';
import { RedirectTo } from './RedirectTo';

/**
 * "Canchas libres" and "Torneos" used to be two top-level sections; both now
 * live under `/events`. The old addresses stay valid (bookmarks, links already
 * sent) and keep their query string, which carries `?event=`.
 */
export const eventRedirectRoutes: RouteObject[] = [
  { path: '/events', element: <RedirectTo to={() => '/events/open-play'} /> },
  { path: '/open-play', element: <RedirectTo to={() => '/events/open-play'} /> },
  { path: '/tournaments', element: <RedirectTo to={() => '/events/tournaments'} /> },
  {
    path: '/tournaments/:tournamentId',
    element: <RedirectTo to={({ tournamentId }) => `/events/tournaments/${tournamentId ?? ''}`} />,
  },
];
