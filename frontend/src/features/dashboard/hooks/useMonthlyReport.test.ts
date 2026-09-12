import { renderHook, waitFor } from '@testing-library/react';
import { useMonthlyReport } from './useMonthlyReport';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { MonthlyReportResponse } from '@/shared/types/api.types';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/dashboard.api', () => ({
  dashboardApi: {
    getMonthlyReport: vi.fn().mockResolvedValue({
      report: {
        month: 3,
        year: 2026,
        by_court: [],
        previous_totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
        by_method: {},
        totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
      },
    } satisfies MonthlyReportResponse),
  },
}));

describe('useMonthlyReport', () => {
  it('returns undefined data when complexId is null', () => {
    const { result } = renderHook(() => useMonthlyReport(null, 3, 2026), {
      wrapper: createQueryWrapper(),
    });
    expect(result.current.data).toBeUndefined();
    expect(result.current.fetchStatus).toBe('idle');
  });

  it('uses correct query key structure', () => {
    const complexId = 'test-complex-id';
    const month = 3;
    const year = 2026;

    const expectedKey = [...queryKeys.dashboard.stats(complexId), 'monthly-report', month, year];

    expect(expectedKey).toEqual(['dashboard', 'stats', 'test-complex-id', 'monthly-report', 3, 2026]);
  });

  it('fetches data when complexId is provided', async () => {
    const { result } = renderHook(() => useMonthlyReport('test-complex-id', 3, 2026), {
      wrapper: createQueryWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual({
      month: 3,
      year: 2026,
      by_court: [],
      previous_totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
      by_method: {},
      totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
    });
  });
});
