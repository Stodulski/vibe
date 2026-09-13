import { createElement, type ReactElement, type ReactNode } from 'react';
import { vi } from 'vitest';
import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider, type UseQueryResult } from '@tanstack/react-query';
import { useMonthlyReport } from '@/features/dashboard';
import type { MonthlyReport } from '@/shared/types/api.types';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: () => ({ selectedComplexId: 'test-complex-id' }),
}));

vi.mock('@/features/dashboard/hooks/useMonthlyReport');
vi.mock('@/features/dashboard/api/dashboard.api');

export const mockedUseMonthlyReport = vi.mocked(useMonthlyReport);

/**
 * `ReportsPage` reads/writes the month and year through `useSearchParams`
 * (see `useMonthYearSelection`), which throws outside a Router — every test
 * that renders the page needs this instead of the bare `render`. It also
 * needs a `QueryClientProvider` now: JOB-06's `useReportExport` polls the
 * export job's status through a real `useQuery`, unlike `useMonthlyReport`
 * (mocked away above) which never touches the real client in these tests.
 */
export function renderReportsPage(ui: ReactElement, initialEntries: string[] = ['/reports']) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });
  // Plain .ts file (no JSX loader here), so the wrapper is built with
  // createElement instead of JSX — see useComplexPageState.test.ts.
  function Wrapper({ children }: { children: ReactNode }) {
    return createElement(
      QueryClientProvider,
      { client: queryClient },
      createElement(MemoryRouter, { initialEntries }, children),
    );
  }
  return render(ui, { wrapper: Wrapper });
}

export function mockHookReturn(overrides: Partial<UseQueryResult<MonthlyReport>> = {}): UseQueryResult<MonthlyReport> {
  return {
    data: undefined,
    error: null,
    isError: false,
    isLoading: false,
    isLoadingError: false,
    isRefetchError: false,
    isSuccess: false,
    isPending: true,
    isRefetching: false,
    isFetching: false,
    isFetched: false,
    isFetchedAfterMount: false,
    isStale: false,
    isPlaceholderData: false,
    status: 'pending',
    fetchStatus: 'idle',
    dataUpdatedAt: 0,
    errorUpdatedAt: 0,
    failureCount: 0,
    failureReason: null,
    errorUpdateCount: 0,
    refetch: vi.fn(),
    ...overrides,
  } as UseQueryResult<MonthlyReport>;
}
