import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { monthlyReportResponseSchema } from './reports.schema';

describe('monthlyReportResponseSchema', () => {
  it('validates dashboardApi.getMonthlyReport response shape', () => {
    const totals = { count: 10, total: 100000, service_fees: 5000, refunded: 2000, net: 93000 };
    const result = monthlyReportResponseSchema.safeParse({
      report: {
        month: 3,
        year: 2026,
        by_method: { cash: { count: 5, total: 50000, refunded: 0, net: 50000 } },
        by_court: [{ court_id: 'ct1', court_name: 'Cancha 1', count: 10, total: 100000, refunded: 2000, net: 93000 }],
        totals,
        previous_totals: totals,
      },
    });
    expect(result.success).toBe(true);
  });
});
