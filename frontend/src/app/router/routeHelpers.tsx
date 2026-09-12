import { lazy, Suspense } from 'react';
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

export function lazyPage(
  factory: () => Promise<{ default: React.ComponentType }>,
  loader: React.ReactNode = <PublicPageLoader />,
) {
  const Component = lazyRetry(factory);
  return (
    <ErrorBoundary>
      <Suspense fallback={loader}>
        <Component />
      </Suspense>
    </ErrorBoundary>
  );
}

export function ownerPage(
  factory: () => Promise<{ default: React.ComponentType }>,
  loader: React.ReactNode = <PageLoader />,
) {
  const Component = lazyRetry(factory);
  return (
    <ErrorBoundary>
      <Suspense fallback={loader}>
        <Component />
      </Suspense>
    </ErrorBoundary>
  );
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
