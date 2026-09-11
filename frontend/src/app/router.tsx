import { createBrowserRouter, type RouteObject } from 'react-router-dom';
import { authRoutes } from './router/authRoutes';
import { ownerStandaloneRoutes, ownerDashboardRoutes } from './router/ownerRoutes';
import { adminRoutes } from './router/adminRoutes';
import { publicRoutes } from './router/publicRoutes';
import { NotFoundPage } from './router/NotFoundPage';
import { RootRedirect } from './router/RootRedirect';
import { ScrollToTop } from './router/ScrollToTop';
import { RouteErrorPage } from './router/RouteErrorPage';

// Every route lives under one pathless parent so that a single
// `ScrollRestoration` covers all of them — see `ScrollToTop`.
const routes: RouteObject[] = [
  // ─── Root redirect ───
  {
    path: '/',
    element: <RootRedirect />,
  },

  // ─── Auth (standalone, no layout) ───
  ...authRoutes,

  // ─── Standalone protected pages (no DashboardLayout) ───
  ...ownerStandaloneRoutes,

  // ─── Owner panel (protected + DashboardLayout) ───
  ...ownerDashboardRoutes,

  // ─── Admin panel (protected + AdminLayout, superadmin only) ───
  ...adminRoutes,

  // ─── Public booking (PublicLayout) ───
  ...publicRoutes,

  // ─── 404 catch-all ───
  {
    path: '*',
    element: <NotFoundPage />,
  },
];

// `errorElement` on this root route is the route-boundary counterpart to
// `routeHelpers.tsx`'s per-page `<ErrorBoundary>`: it catches render errors
// thrown by anything that boundary doesn't wrap — layouts, `ProtectedRoute`/
// `GuestRoute`, `RootRedirect`, `NotFoundPage`, `ScrollToTop` — instead of
// leaving a blank screen (see `RouteErrorPage`).
export const router = createBrowserRouter([
  { element: <ScrollToTop />, errorElement: <RouteErrorPage />, children: routes },
]);
