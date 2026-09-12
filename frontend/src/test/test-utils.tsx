import { createContext, useContext, type ReactElement, type ReactNode } from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';

/**
 * `gcTime: 0` drops a query's cache entry the instant it has no observers,
 * instead of the library default (5 min) — without it, a query created in
 * one test can still be sitting in cache (and get read by `getQueryData`,
 * silently reused, or leak into an assertion) when the next test's `render`
 * mounts a component subscribing to the same key. `retry: false` is kept for
 * the same reason it always was: a query/mutation under test should fail on
 * the first rejected request, not spend `waitFor`'s timeout budget on
 * React Query's own retry backoff.
 */
function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
}

interface WrapperProps {
  children: React.ReactNode;
}

/**
 * How the component under test reaches the router's only route.
 *
 * `createMemoryRouter` takes its routes once, at construction, so the element
 * cannot simply be `children` — every rerender would build a new router and
 * remount the tree, losing whatever state the test had just typed into it.
 * The router is built once per wrapper and reads the current children from
 * here instead.
 */
const TestChildrenContext = createContext<ReactNode>(null);

/**
 * A data router, not `<MemoryRouter>`: `useBlocker` (and every other data-router
 * hook) throws "must be used within a data router" under the declarative one,
 * and the app itself runs on `createBrowserRouter` — so a test rendering a form
 * that guards its unsaved changes was testing a router the app does not use.
 */
export function createWrapper(initialEntries?: string[]) {
  const queryClient = createTestQueryClient();
  // Declared here rather than at module scope: a top-level component in a file
  // of helpers trips react-refresh/only-export-components, and this one exists
  // only to be this router's single route element.
  const RouteSlot = () => <>{useContext(TestChildrenContext)}</>;
  const router = createMemoryRouter([{ path: '*', element: <RouteSlot /> }], {
    initialEntries: initialEntries ?? ['/'],
  });
  return function Wrapper({ children }: WrapperProps) {
    return (
      <QueryClientProvider client={queryClient}>
        <TestChildrenContext.Provider value={children}>
          <RouterProvider router={router} />
        </TestChildrenContext.Provider>
      </QueryClientProvider>
    );
  };
}

/**
 * The wrapper for tests that render a React Query hook with `renderHook` and
 * need no router — most hook tests under `src/features/*\/hooks`. Reuses the
 * same `gcTime: 0`/`retry: false` `QueryClient` as {@link createWrapper}, so
 * hook tests and component tests share one definition of "a clean test query
 * client" instead of each file redeclaring it locally.
 */
export function createQueryWrapper() {
  const queryClient = createTestQueryClient();
  return function QueryWrapper({ children }: WrapperProps) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

export function renderWithProviders(
  ui: ReactElement,
  options?: Omit<RenderOptions, 'wrapper'> & { initialEntries?: string[] },
) {
  const { initialEntries, ...renderOptions } = options ?? {};
  return render(ui, {
    wrapper: createWrapper(initialEntries),
    ...renderOptions,
  });
}

export { screen, waitFor, within } from '@testing-library/react';
export { default as userEvent } from '@testing-library/user-event';
