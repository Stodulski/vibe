import { describe, it, expect, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { mockedUseMonthlyReport, mockHookReturn, renderReportsPage } from './reports-page-test-helpers';
import ReportsPage from './ReportsPage';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MONTH_NAMES } from './reports/constants';

describe('ReportsPage', () => {
  beforeEach(() => {
    mockedUseMonthlyReport.mockReturnValue(mockHookReturn());
  });

  it('renders page title "Reportes"', () => {
    renderReportsPage(<ReportsPage />);
    expect(screen.getByText('Reportes')).toBeInTheDocument();
  });

  // Asserted on what the trigger SHOWS, not on a `value` attribute. These are
  // the app's Select now rather than the browser's, and its trigger is a
  // button displaying the chosen option — `toHaveValue` only ever worked
  // because the control used to be a native <select>.
  it('opens on the current month', () => {
    renderReportsPage(<ReportsPage />);

    const trigger = screen.getByLabelText(ES_AR.reports.month);
    expect(trigger).toHaveTextContent(MONTH_NAMES[new Date().getMonth()] ?? '');
  });

  it('opens on the current year', () => {
    renderReportsPage(<ReportsPage />);

    expect(screen.getByLabelText(ES_AR.reports.year)).toHaveTextContent(String(new Date().getFullYear()));
  });

  it('renders "Descargar Excel" button', () => {
    renderReportsPage(<ReportsPage />);
    expect(screen.getByText('Descargar Excel')).toBeInTheDocument();
  });

  it('opens the month named by ?month= in the URL, surviving a refresh', () => {
    // January is always a valid choice for the current year regardless of
    // when this test runs, unlike an arbitrary later month.
    renderReportsPage(<ReportsPage />, ['/reports?month=1']);
    const trigger = screen.getByLabelText(ES_AR.reports.month);
    expect(trigger).toHaveTextContent(MONTH_NAMES[0]);
  });

  it('falls back to the current month for an invalid ?month=', () => {
    renderReportsPage(<ReportsPage />, ['/reports?month=not-a-month']);
    const trigger = screen.getByLabelText(ES_AR.reports.month);
    expect(trigger).toHaveTextContent(MONTH_NAMES[new Date().getMonth()] ?? '');
  });
});

describe('ReportsPage states', () => {
  beforeEach(() => {
    mockedUseMonthlyReport.mockReturnValue(mockHookReturn());
  });

  it('shows a loading skeleton, not a centered spinner, when isLoading is true', () => {
    mockedUseMonthlyReport.mockReturnValue(mockHookReturn({ isLoading: true }));
    const { container } = renderReportsPage(<ReportsPage />);
    // UI-06: this used to be a centered spinner that collapsed the panel's height.
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
  });

  it('shows empty state when report has no data', () => {
    const emptyReport = {
      month: 3,
      year: 2026,
      by_court: [],
      previous_totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
      by_method: {},
      totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
    };
    mockedUseMonthlyReport.mockReturnValue(
      mockHookReturn({
        data: emptyReport,
        isLoading: false,
        isSuccess: true,
        status: 'success',
        isPending: false,
      }),
    );
    renderReportsPage(<ReportsPage />);
    expect(screen.getByText('Sin datos')).toBeInTheDocument();
    expect(screen.getByText(/No hay pagos registrados/)).toBeInTheDocument();
  });

  it('shows error state when API fails', () => {
    mockedUseMonthlyReport.mockReturnValue(
      mockHookReturn({
        isError: true,
        isLoading: false,
        status: 'error',
        error: new Error('fetch failed'),
      }),
    );
    renderReportsPage(<ReportsPage />);
    expect(screen.getByText('Error al cargar el reporte')).toBeInTheDocument();
  });
});
