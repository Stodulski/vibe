import { describe, it, expect, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { mockedUseMonthlyReport, mockHookReturn, renderReportsPage } from './reports-page-test-helpers';
import ReportsPage from './ReportsPage';

describe('ReportsPage table rendering', () => {
  beforeEach(() => {
    mockedUseMonthlyReport.mockReturnValue(mockHookReturn());
  });

  it('shows report table with method rows when data is available', () => {
    const report = {
      month: 3,
      year: 2026,
      by_court: [],
      previous_totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
      by_method: {
        mercadopago: { count: 10, total: 500000, refunded: 50000, net: 450000 },
        cash: { count: 5, total: 200000, refunded: 0, net: 200000 },
      },
      totals: {
        count: 15,
        total: 700000,
        service_fees: 35000,
        refunded: 50000,
        net: 650000,
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

    expect(screen.getAllByText('MercadoPago').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Efectivo').length).toBeGreaterThanOrEqual(1);
    const totalElements = screen.getAllByText('Total');
    expect(totalElements.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('15')).toBeInTheDocument();
  });
});
