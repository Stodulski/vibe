import { renderHook, waitFor, act } from '@testing-library/react';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
// Imported at the top, not with `await import()` inside each test: `vi.mock`
// is hoisted above these either way, so the dynamic form bought nothing — and
// it charged the whole module graph's transform (Babel, since the React
// Compiler was turned on) to the first test's 10s budget, which is what made
// this file look flaky.
import { dashboardApi } from '@/features/dashboard/api/dashboard.api';
import { useReportExport } from './useReportExport';

vi.mock('@/features/dashboard/api/dashboard.api', () => ({
  dashboardApi: {
    exportPaymentsExcel: vi.fn(),
  },
}));

describe('useReportExport — maps report_export_* codes instead of a generic fallback', () => {
  afterEach(() => {
    vi.mocked(dashboardApi.exportPaymentsExcel).mockReset();
  });

  it('shows the "too large" message when the server refuses report_export_too_large', async () => {
    const backendError = await makeConsumedHttpError(422, { error: 'report_export_too_large' });
    vi.mocked(dashboardApi.exportPaymentsExcel).mockRejectedValueOnce(backendError);

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
    const backendError = await makeConsumedHttpError(503, { error: 'report_export_timed_out' });
    vi.mocked(dashboardApi.exportPaymentsExcel).mockRejectedValueOnce(backendError);

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
    const backendError = await makeConsumedHttpError(500, {});
    vi.mocked(dashboardApi.exportPaymentsExcel).mockRejectedValueOnce(backendError);

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
