import { isValidElement } from 'react';
import type { RouteObject } from 'react-router-dom';
import { router } from './router';
import { RouteErrorPage } from './router/RouteErrorPage';
import { DashboardLayout } from '@/app/layout/DashboardLayout';
import { AdminLayout } from '@/app/layout/AdminLayout';
import { PublicLayout } from '@/shared/components/layout/PublicLayout';
import { NotFoundPage } from './router/NotFoundPage';
import { ProtectedRoute } from './router/ProtectedRoute';

/** Every component rendered eagerly by the route tree — lazy ones are `object` types, not functions. */
function eagerComponents(node: unknown, found = new Set<unknown>()): Set<unknown> {
  if (Array.isArray(node)) {
    for (const child of node as unknown[]) eagerComponents(child, found);
    return found;
  }
  if (!isValidElement(node)) return found;
  if (typeof node.type === 'function') found.add(node.type);
  const { children } = node.props as { children?: unknown };
  if (children !== undefined) eagerComponents(children, found);
  return found;
}

function walkRoutes(routes: RouteObject[], found = new Set<unknown>()): Set<unknown> {
  for (const route of routes) {
    eagerComponents(route.element, found);
    if (route.children) walkRoutes(route.children, found);
  }
  return found;
}

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

  it('code-splits the app shell so no layout ships in the entry chunk', () => {
    // The three layouts and `NotFoundPage` used to be static imports in the
    // route modules, so an anonymous visitor of `/:slug` downloaded the owner
    // and admin layouts too. `lazyShell` puts each behind its own chunk; a
    // lazy component's element `type` is React's lazy *object*, never the
    // component function, so a static import reappearing here fails this.
    const eager = walkRoutes(router.routes);
    // Proves the walk actually reaches route elements before asserting absences.
    expect(eager).toContain(ProtectedRoute);
    expect(eager).not.toContain(DashboardLayout);
    expect(eager).not.toContain(AdminLayout);
    expect(eager).not.toContain(PublicLayout);
    expect(eager).not.toContain(NotFoundPage);
  });
});
