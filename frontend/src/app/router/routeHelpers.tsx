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
