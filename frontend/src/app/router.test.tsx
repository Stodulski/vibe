import { describe, it, expect } from 'vitest';
import { isValidElement } from 'react';
import { matchRoutes, type RouteObject } from 'react-router-dom';
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

/** Route paths the router matches for a URL, outermost first. */
function matchedPaths(url: string): (string | undefined)[] {
  return (matchRoutes(router.routes as RouteObject[], url) ?? []).map((match) => match.route.path);
}

describe('public complex URLs', () => {
  // Every explicit single-segment platform route. The legacy `/:slug`
  // redirect must never take one of these, or a complex whose slug matched
  // a platform word would hijack the screen (react-router ranks static
  // segments above dynamic ones; this pins that for the whole app).
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
    // Multi-segment platform routes whose second segment is a plain word,
    // the shape a legacy `/:slug/book` would have to share to collide.
    '/cash/sell',
    '/settings/mp/callback',
    '/register/google',
    '/auth/google/callback',
  ])('serves the platform route %s, not a legacy complex redirect', (path) => {
    const paths = matchedPaths(path);

    expect(paths).toContain(path);
    expect(paths).not.toContain('/:slug');
    expect(paths).not.toContain('/:slug/book');
    expect(paths).not.toContain('/:slug/book/confirm');
    expect(paths).not.toContain('/:slug/book/success');
    expect(paths).not.toContain('/:slug/book/cancel');
  });

  it.each([
    ['/los-alamos', '/:slug'],
    ['/los-alamos/book', '/:slug/book'],
    ['/los-alamos/book/confirm', '/:slug/book/confirm'],
    ['/los-alamos/book/success', '/:slug/book/success'],
    ['/los-alamos/book/cancel', '/:slug/book/cancel'],
  ])('sends the legacy complex URL %s to its redirect %s', (url, redirect) => {
    expect(matchedPaths(url)).toContain(redirect);
  });

  it('serves the complex storefront under /c/:slug', () => {
    expect(matchedPaths('/c/los-alamos')).toContain('/c/:slug');
  });
});
