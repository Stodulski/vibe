import type { ReactElement } from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';

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

export function createWrapper(initialEntries?: string[]) {
  const queryClient = createTestQueryClient();
  return function Wrapper({ children }: WrapperProps) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={initialEntries ?? ['/']}>{children}</MemoryRouter>
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
