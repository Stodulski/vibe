import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { makeConsumedHttpError } from '@/test/factories';
import { mockedUseMonthlyReport, mockHookReturn, renderReportsPage } from './reports-page-test-helpers';
import ReportsPage from './ReportsPage';

vi.mock('@/features/dashboard/api/dashboard.api');

describe('ReportsPage export', () => {
  beforeEach(() => {
    mockedUseMonthlyReport.mockReturnValue(mockHookReturn());
  });

  it('button shows "Descargando..." with spinner during the synchronous fallback', async () => {
    // A bare 404 from the async job route (JOB-06) is what an older/legacy
    // backend answers — `useReportExport` reads that as "fall back to the
    // synchronous export" rather than a user-facing error (see
    // `useReportExport.ts`'s `isLegacyExportRoute`).
    const { dashboardApi } = await import('@/features/dashboard/api/dashboard.api');
    vi.mocked(dashboardApi.requestPaymentsExport).mockRejectedValue(await makeConsumedHttpError(404, null));

    let resolveExport!: (value: Blob) => void;
    const exportPromise = new Promise<Blob>((resolve) => {
      resolveExport = resolve;
    });
    vi.mocked(dashboardApi.exportPaymentsExcel).mockReturnValue(exportPromise);

    mockedUseMonthlyReport.mockReturnValue(mockHookReturn({ isLoading: false }));

    renderReportsPage(<ReportsPage />);

    const button = screen.getByText('Descargar Excel');
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText('Descargando...')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Descargando/ })).toHaveAttribute('aria-busy', 'true');

    resolveExport(new Blob());
  });
});
