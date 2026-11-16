import type { RouteObject } from 'react-router-dom';
import { RedirectTo } from './RedirectTo';

/**
 * "Canchas abiertas" and "Torneos" used to be separate pages; both now live on
 * `/events`. The old addresses stay valid (bookmarks, links already sent) and
 * keep their query string, which carries `?event=`.
 */
export const eventRedirectRoutes: RouteObject[] = [
  { path: '/events/open-play', element: <RedirectTo to={() => '/events'} /> },
  { path: '/events/tournaments', element: <RedirectTo to={() => '/events'} /> },
  { path: '/open-play', element: <RedirectTo to={() => '/events'} /> },
  { path: '/tournaments', element: <RedirectTo to={() => '/events'} /> },
  {
    path: '/tournaments/:tournamentId',
    element: <RedirectTo to={({ tournamentId }) => `/events/tournaments/${tournamentId ?? ''}`} />,
  },
];
