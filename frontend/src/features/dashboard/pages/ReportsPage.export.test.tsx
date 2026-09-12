import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent } from '@testing-library/react';
import { mockedUseMonthlyReport, mockHookReturn, renderReportsPage } from './reports-page-test-helpers';
import ReportsPage from './ReportsPage';

vi.mock('@/features/dashboard/api/dashboard.api');

describe('ReportsPage export', () => {
  beforeEach(() => {
    mockedUseMonthlyReport.mockReturnValue(mockHookReturn());
  });

  it('button shows "Descargando..." with spinner when exporting', async () => {
    let resolveExport!: (value: Blob) => void;
    const exportPromise = new Promise<Blob>((resolve) => {
      resolveExport = resolve;
    });

    const { dashboardApi } = await import('@/features/dashboard/api/dashboard.api');
    vi.mocked(dashboardApi.exportPaymentsExcel).mockReturnValue(exportPromise);

    mockedUseMonthlyReport.mockReturnValue(mockHookReturn({ isLoading: false }));

    renderReportsPage(<ReportsPage />);

    const button = screen.getByText('Descargar Excel');
    fireEvent.click(button);

    expect(screen.getByText('Descargando...')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Descargando/ })).toHaveAttribute('aria-busy', 'true');

    resolveExport(new Blob());
  });
});
