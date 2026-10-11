import { describe, it, expect } from 'vitest';
import { isValidElement } from 'react';
import { matchRoutes, type RouteObject } from 'react-router-dom';
import { router } from './router';
import { RouteErrorPage } from './router/RouteErrorPage';
import { DashboardLayout } from '@/app/layout/DashboardLayout';
import { AdminLayout } from '@/app/layout/AdminLayout';
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
    // `<ErrorBoundary>` never covered `DashboardLayout`/`AdminLayout`,
    // `ProtectedRoute`/`GuestRoute`, `RootRedirect`, `NotFoundPage`, or
    // `ScrollToTop` — a render error there landed on a blank screen. This fails
    // if `errorElement` is ever removed from `router.tsx`'s root route (see
    // `RouteErrorPage.test.tsx` for proof the mechanism itself renders the
    // fallback and hides the erroring subtree).
    const [rootRoute] = router.routes as [RouteObject];
    expect(rootRoute.errorElement).toEqual(<RouteErrorPage />);
  });

  it('code-splits the app shell so no layout ships in the entry chunk', () => {
    // The layouts and `NotFoundPage` used to be static imports in the route
    // modules, so an anonymous visitor of any page downloaded the owner and
    // admin layouts too. `lazyShell` puts each behind its own chunk; a lazy
    // component's element `type` is React's lazy *object*, never the component
    // function, so a static import reappearing here fails this.
    const eager = walkRoutes(router.routes);
    // Proves the walk actually reaches route elements before asserting absences.
    expect(eager).toContain(ProtectedRoute);
    expect(eager).not.toContain(DashboardLayout);
    expect(eager).not.toContain(AdminLayout);
    expect(eager).not.toContain(NotFoundPage);
  });
});

/** Route paths the router matches for a URL, outermost first. */
function matchedPaths(url: string): (string | undefined)[] {
  return (matchRoutes(router.routes as RouteObject[], url) ?? []).map((match) => match.route.path);
}

describe('platform routes', () => {
  it.each([
    '/login',
    '/register',
    '/verify-email',
    '/verify-email-sent',
    '/confirm-email-change',
    '/forgot-password',
    '/reset-password',
    '/onboarding',
    '/dashboard',
    '/bookings',
    '/cash',
    '/courts',
    '/clients',
    '/reports',
    '/settings',
    '/profile',
    '/admin',
    '/cash/sell',
    '/settings/mp/callback',
    '/register/google',
    '/auth/google/callback',
  ])('serves the platform route %s', (path) => {
    expect(matchedPaths(path)).toContain(path);
  });
});

describe('retired public URLs', () => {
  // The app serves no storefronts: a complex's public page lives on the landing
  // (vibe.com.ar). Its old addresses, `/c/*` and the pre-`/c/` `/<slug>` paths,
  // are not redirected. They fall through to the 404 catch-all.
  it.each([
    '/c/los-alamos',
    '/c/los-alamos/book',
    '/c/los-alamos/book/confirm',
    '/c/los-alamos/book/success',
    '/c/los-alamos/book/cancel',
    '/canchas/banfield',
    '/los-alamos',
    '/los-alamos/book/confirm',
  ])('sends %s to the 404 catch-all', (url) => {
    expect(matchedPaths(url).at(-1)).toBe('*');
  });
});
