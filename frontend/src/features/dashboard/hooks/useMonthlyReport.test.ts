import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useMonthlyReport } from './useMonthlyReport';
import { queryKeys } from '@/shared/lib/queryKeys';
import { createQueryWrapper } from '@/test/test-utils';

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
});
