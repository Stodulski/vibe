import { lazy, Suspense } from 'react';
import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { ErrorBoundary } from '@/shared/components/common/ErrorBoundary';
import { safeSessionStorage } from '@/shared/lib/safeStorage';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { CashSellSkeleton } from '@/features/cash/components/skeletons/CashSellSkeleton';
import { CashSummarySkeleton } from '@/features/cash/components/skeletons/CashSummarySkeleton';
import { ProductDetailSkeleton } from '@/features/products/components/skeletons/ProductDetailSkeleton';
import { ProductListSkeleton } from '@/features/products/components/skeletons/ProductListSkeleton';
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

/** Mobile-only title line: `PageHeader` shows its title below `md` only (the masthead carries it above). */
function titlePlaceholder(width = 'w-32') {
  return <Skeleton className={`mb-3 h-7 rounded-lg sm:mb-5 md:hidden ${width}`} />;
}

/** Mirrors `CashSectionTabs`: three equal tabs below `sm`, content-sized from `sm`, on a hairline. */
function cashTabsPlaceholder() {
  return (
    <div className="-mx-3 mb-3 px-3 sm:-mx-4 sm:mb-4 sm:px-4">
      <div className="border-border-subtle flex gap-1 border-b">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex h-11 flex-1 items-center justify-center px-3 sm:flex-none">
            <Skeleton className="h-4 w-14 rounded-lg" />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Back link row shared by the cash session and product detail pages. */
function backLinkPlaceholder() {
  return (
    <div className="flex h-5 items-center gap-1.5">
      <Skeleton className="size-4 rounded" />
      <Skeleton className="h-4 w-28 rounded-lg" />
    </div>
  );
}

/** Cold-load fallback for `/cash`: title, tabs, then the shift summary. */
export function cashFallback() {
  return (
    <div>
      {titlePlaceholder()}
      {cashTabsPlaceholder()}
      <CashSummarySkeleton />
    </div>
  );
}

/** Cold-load fallback for `/cash/sell`. */
export function cashSellFallback() {
  return (
    <div>
      {titlePlaceholder()}
      {cashTabsPlaceholder()}
      <CashSellSkeleton />
    </div>
  );
}

/** Cold-load fallback for `/cash/sessions/:id`: back link, title, closed-session summary. */
export function cashSessionFallback() {
  return (
    <div className="max-w-3xl space-y-6">
      {backLinkPlaceholder()}
      {titlePlaceholder('w-40')}
      <CashSummarySkeleton showActions={false} />
    </div>
  );
}

/** Cold-load fallback for `/cash/products`: title + create button, tabs, search row, catalog. */
export function productsFallback() {
  return (
    <div>
      <div className="mb-3 flex flex-col gap-2.5 sm:mb-5 sm:flex-row sm:items-center sm:justify-between">
        <Skeleton className="h-7 w-32 rounded-lg md:hidden" />
        <Skeleton className="h-10 w-full rounded-xl sm:w-40 md:ml-auto" />
      </div>
      {cashTabsPlaceholder()}
      <div className="space-y-2.5 sm:space-y-4">
        <Skeleton className="h-11 w-full rounded-xl" />
        <ProductListSkeleton />
      </div>
    </div>
  );
}

/** Cold-load fallback for `/cash/products/:id`: back link, title, detail. */
export function productDetailFallback() {
  return (
    <div className="space-y-4">
      {backLinkPlaceholder()}
      {titlePlaceholder('w-40')}
      <ProductDetailSkeleton />
    </div>
  );
}
