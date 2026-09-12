import { describe, it, expect, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { mockedUseMonthlyReport, mockHookReturn, renderReportsPage } from './reports-page-test-helpers';
import ReportsPage from './ReportsPage';
import { ES_AR } from '@/shared/i18n/es_AR';

/** The report's columns, in the order a reader scans them. */
const COLUMNS = [
  ES_AR.reports.method,
  ES_AR.reports.count,
  ES_AR.reports.total,
  ES_AR.reports.refunded,
  ES_AR.reports.net,
];

describe('ReportsPage table accessibility and fees', () => {
  beforeEach(() => {
    mockedUseMonthlyReport.mockReturnValue(mockHookReturn());
  });

  it('table has proper accessibility attributes', () => {
    const report = {
      month: 3,
      year: 2026,
      by_court: [],
      previous_totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
      by_method: {
        cash: { count: 5, total: 200000, refunded: 0, net: 200000 },
      },
      totals: { count: 5, total: 200000, service_fees: 0, refunded: 0, net: 200000 },
    };
    mockedUseMonthlyReport.mockReturnValue(
      mockHookReturn({
        data: report,
        isLoading: false,
        isSuccess: true,
        status: 'success',
        isPending: false,
      }),
    );
    renderReportsPage(<ReportsPage />);

    const table = screen.getByRole('table');
    expect(table).toHaveAttribute('aria-label');
    // The labels, not the count. Five headers would still be five after a
    // column was renamed or two were swapped — and on a revenue report, a
    // reader mistaking "Reembolsado" for "Neto" is the whole problem.
    expect(screen.getAllByRole('columnheader').map((h) => h.textContent)).toEqual(COLUMNS);
  });

  it('shows service fees note when service_fees > 0', () => {
    const report = {
      month: 3,
      year: 2026,
      by_court: [],
      previous_totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
      by_method: {
        mercadopago: { count: 10, total: 500000, refunded: 50000, net: 450000 },
      },
      totals: {
        count: 10,
        total: 500000,
        service_fees: 35000,
        refunded: 50000,
        net: 450000,
      },
    };
    mockedUseMonthlyReport.mockReturnValue(
      mockHookReturn({
        data: report,
        isLoading: false,
        isSuccess: true,
        status: 'success',
        isPending: false,
      }),
    );
    renderReportsPage(<ReportsPage />);
    expect(screen.getAllByText(/Comisiones de servicio/).length).toBeGreaterThanOrEqual(1);
  });
});
