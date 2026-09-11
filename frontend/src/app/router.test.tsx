import type { RouteObject } from 'react-router-dom';
import { router } from './router';
import { RouteErrorPage } from './router/RouteErrorPage';

describe('router', () => {
  it('wires an errorElement on the root route, not just on individual lazy pages', () => {
    // Regression guard for the fix itself: `routeHelpers.tsx`'s per-page
    // `<ErrorBoundary>` never covered `DashboardLayout`/`AdminLayout`/
    // `PublicLayout`, `ProtectedRoute`/`GuestRoute`, `RootRedirect`,
    // `NotFoundPage`, or `ScrollToTop` — a render error there landed on a
    // blank screen. This fails if `errorElement` is ever removed from
    // `router.tsx`'s root route (see `RouteErrorPage.test.tsx` for proof the
    // mechanism itself renders the fallback and hides the erroring subtree).
    const [rootRoute] = router.routes as [RouteObject];
    expect(rootRoute.errorElement).toEqual(<RouteErrorPage />);
  });
});
