import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('@/features/dashboard/api/dashboard.api', () => ({
  dashboardApi: {
    exportPaymentsExcel: vi.fn(),
  },
}));

describe('useReportExport — maps report_export_* codes instead of a generic fallback', () => {
  afterEach(async () => {
    const { dashboardApi } = await import('@/features/dashboard/api/dashboard.api');
    vi.mocked(dashboardApi.exportPaymentsExcel).mockReset();
  });

  it('shows the "too large" message when the server refuses report_export_too_large', async () => {
    const { dashboardApi } = await import('@/features/dashboard/api/dashboard.api');
    const backendError = await makeConsumedHttpError(422, { error: 'report_export_too_large' });
    vi.mocked(dashboardApi.exportPaymentsExcel).mockRejectedValueOnce(backendError);

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026));

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).not.toBeNull();
    });
    expect(result.current.exportError).toBe(ES_AR.dashboard.exportTooLarge);
  });

  it('shows the "timed out" message when the server refuses report_export_timed_out', async () => {
    const { dashboardApi } = await import('@/features/dashboard/api/dashboard.api');
    const backendError = await makeConsumedHttpError(503, { error: 'report_export_timed_out' });
    vi.mocked(dashboardApi.exportPaymentsExcel).mockRejectedValueOnce(backendError);

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026));

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).not.toBeNull();
    });
    expect(result.current.exportError).toBe(ES_AR.dashboard.exportTimedOut);
  });

  it('falls back to the generic message for an unrecognized code', async () => {
    const { dashboardApi } = await import('@/features/dashboard/api/dashboard.api');
    const backendError = await makeConsumedHttpError(500, {});
    vi.mocked(dashboardApi.exportPaymentsExcel).mockRejectedValueOnce(backendError);

    const { useReportExport } = await import('./useReportExport');
    const { result } = renderHook(() => useReportExport('c1', 3, 2026));

    await act(async () => {
      await result.current.handleExport();
    });

    await waitFor(() => {
      expect(result.current.exportError).not.toBeNull();
    });
    expect(result.current.exportError).toBe(ES_AR.dashboard.exportGenericError);
  });
});
