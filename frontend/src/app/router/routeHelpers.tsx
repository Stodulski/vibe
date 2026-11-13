import { lazy, Suspense } from 'react';
import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { ErrorBoundary } from '@/shared/components/common/ErrorBoundary';
import { safeSessionStorage } from '@/shared/lib/safeStorage';
import { PageLoader, PublicPageLoader } from './loaders';

export function lazyRetry(factory: () => Promise<{ default: React.ComponentType }>) {
  return lazy(() =>
    factory().catch(() => {
      // Chunk missing after deploy — reload once to pick up new assets.
      const key = 'chunk_reload';
      if (!safeSessionStorage.get(key)) {
        safeSessionStorage.set(key, '1');
        window.location.reload();
        // Intentionally never resolves — the page is about to reload.
        return new Promise(() => {
          /* noop */
        });
      }
      safeSessionStorage.remove(key);
      return factory();
    }),
  );
}

/**
 * One route's subtree: its own error boundary, its own Suspense fallback, and
 * its own query-error reset scope.
 *
 * The reset scope is per section on purpose. `QueryErrorResetBoundary` clears
 * the failed queries *inside* it, so "Reintentar" on the bookings page retries
 * the bookings page's queries and nothing else — a single app-wide scope would
 * have that button also re-run whatever failed on a screen the person left ten
 * minutes ago. Before this the boundary's retry only cleared `hasError`: the
 * cached error was re-thrown on the next render, so the only recovery the app
 * actually offered was `window.location.reload()` in `RouteErrorPage`.
 */
function routePage(factory: () => Promise<{ default: React.ComponentType }>, loader: React.ReactNode) {
  const Component = lazyRetry(factory);
  return (
    <QueryErrorResetBoundary>
      {({ reset }) => (
        <ErrorBoundary onReset={reset}>
          <Suspense fallback={loader}>
            <Component />
          </Suspense>
        </ErrorBoundary>
      )}
    </QueryErrorResetBoundary>
  );
}

export function lazyPage(
  factory: () => Promise<{ default: React.ComponentType }>,
  loader: React.ReactNode = <PublicPageLoader />,
) {
  return routePage(factory, loader);
}

export function ownerPage(
  factory: () => Promise<{ default: React.ComponentType }>,
  loader: React.ReactNode = <PageLoader />,
) {
  return routePage(factory, loader);
}

/**
 * Same lazy + retry treatment for the app shell — the three route layouts
 * and `NotFoundPage` — which used to be static imports, so every anonymous
 * visitor of `/:slug` downloaded the owner and admin layouts too.
 *
 * Deliberately *not* wrapped in `<ErrorBoundary>` the way `lazyPage`/
 * `ownerPage` are: shell render errors are what `router.tsx`'s root
 * `errorElement` exists for (see the comment there), and a boundary here
 * would swallow them before the route boundary ever saw them.
 */
export function lazyShell(
  factory: () => Promise<{ default: React.ComponentType }>,
  loader: React.ReactNode = <PageLoader />,
) {
  const Component = lazyRetry(factory);
  return (
    <Suspense fallback={loader}>
      <Component />
    </Suspense>
  );
}
